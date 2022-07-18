package fraud

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"golang.org/x/sync/errgroup"

	pb "github.com/celestiaorg/celestia-node/fraud/pb"
	"github.com/celestiaorg/celestia-node/params"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

// fetchHeaderTimeout duration of GetByHeight request to fetch an ExtendedHeader.
const fetchHeaderTimeout = time.Minute * 2

var fraudProtocolID = protocol.ID(fmt.Sprintf("/fraud/v0.0.1/%s", params.DefaultNetwork()))

// service is responsible for validating and propagating Fraud Proofs.
// It implements the Service interface.
type service struct {
	topicsLk sync.RWMutex
	topics   map[ProofType]*pubsub.Topic

	storesLk sync.RWMutex
	stores   map[ProofType]datastore.Datastore

	pubsub *pubsub.PubSub
	host   host.Host
	getter headerFetcher
	ds     datastore.Datastore
}

func NewService(p *pubsub.PubSub, host host.Host, getter headerFetcher, ds datastore.Datastore) Service {
	return &service{
		pubsub: p,
		host:   host,
		getter: getter,
		topics: make(map[ProofType]*pubsub.Topic),
		stores: make(map[ProofType]datastore.Datastore),
		ds:     ds,
	}
}

func (f *service) Subscribe(proofType ProofType) (_ Subscription, err error) {
	f.topicsLk.Lock()
	defer f.topicsLk.Unlock()
	t, ok := f.topics[proofType]
	if !ok {
		t, err = join(f.pubsub, proofType, f.processIncoming)
		if err != nil {
			return nil, err
		}
		f.topics[proofType] = t
	}
	subs, err := t.Subscribe()
	if err != nil {
		return nil, err
	}
	return &subscription{subs}, nil
}

func (f *service) Broadcast(ctx context.Context, p Proof) error {
	bin, err := p.MarshalBinary()
	if err != nil {
		return err
	}
	f.topicsLk.RLock()
	t, ok := f.topics[p.Type()]
	f.topicsLk.RUnlock()
	if !ok {
		return fmt.Errorf("fraud: unmarshaler for %s proof is not registered", p.Type())
	}
	return t.Publish(ctx, bin)
}

func (f *service) processIncoming(
	ctx context.Context,
	proofType ProofType,
	from peer.ID,
	msg *pubsub.Message,
) pubsub.ValidationResult {
	proof, err := Unmarshal(proofType, msg.Data)
	if err != nil {
		log.Errorw("unmarshalling failed", "err", err)
		if !errors.Is(err, &errNoUnmarshaler{}) {
			f.pubsub.BlacklistPeer(from)
		}
		return pubsub.ValidationReject
	}

	newCtx, cancel := context.WithTimeout(ctx, fetchHeaderTimeout)
	extHeader, err := f.getter(newCtx, proof.Height())
	defer cancel()
	if err != nil {
		// Timeout means there is a problem with the network.
		// As we cannot prove or discard Fraud Proof, user must restart the node.
		if errors.Is(err, context.DeadlineExceeded) {
			log.Errorw("failed to fetch header. Timeout reached.")
			// TODO(@vgonkivs): add handling for this case. As we are not able to verify fraud proof.
		}
		log.Errorw("failed to fetch header to verify a fraud proof",
			"err", err, "proofType", proof.Type(), "height", proof.Height())
		return pubsub.ValidationIgnore
	}
	err = proof.Validate(extHeader)
	if err != nil {
		log.Errorw("proof validation err: ",
			"err", err, "proofType", proof.Type(), "height", proof.Height())
		f.pubsub.BlacklistPeer(from)
		return pubsub.ValidationReject
	}
	log.Warnw("received fraud proof", "proofType", proof.Type(),
		"height", proof.Height(),
		"hash", hex.EncodeToString(extHeader.DAH.Hash()),
		"from", from.String(),
	)
	msg.ValidatorData = proof
	f.storesLk.Lock()
	store, ok := f.stores[proofType]
	if !ok {
		store = initStore(proofType, f.ds)
		f.stores[proofType] = store
	}
	f.storesLk.Unlock()
	err = put(ctx, store, string(proof.HeaderHash()), msg.Data)
	if err != nil {
		log.Error(err)
	}
	log.Warn("Shutting down services...")
	return pubsub.ValidationAccept
}

func (f *service) Get(ctx context.Context, proofType ProofType) ([]Proof, error) {
	f.storesLk.Lock()
	store, ok := f.stores[proofType]
	if !ok {
		store = initStore(proofType, f.ds)
		f.stores[proofType] = store
	}
	f.storesLk.Unlock()

	return getAll(ctx, store, proofType)
}

func (f *service) sendRequest(
	ctx context.Context,
	p peer.ID, message *pb.FraudMessage,
) ([]Proof, error) {
	stream, err := f.host.NewStream(ctx, p, fraudProtocolID)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := stream.Close(); err != nil {
			log.Warn(err)
		}
	}()

	_, err = serde.Write(stream, message)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, err
	}

	_, err = serde.Read(stream, message)
	if err != nil {
		stream.Reset() //nolint:errcheck
		return nil, err
	}

	if len(message.RespondedProofs) == 0 {
		stream.Reset() //nolint:errcheck
		return nil, ErrProofNotFound
	}

	var (
		proofs = make([]Proof, 0)
		wg     = &sync.WaitGroup{}
		mu     = &sync.Mutex{}
	)
	wg.Add(len(message.RespondedProofs))
	for _, data := range message.RespondedProofs {
		go func(data *pb.RespondedProof) {
			defer wg.Done()
			f.topicsLk.RLock()
			t, ok := f.topics[toProof(data.Type)]
			f.topicsLk.RUnlock()
			if !ok {
				log.Errorf("unmarshaler for proof type %s is not registered", toProof(data.Type))
				return
			}
			proof, err := t.unmarshal(data.Value)
			if err != nil {
				log.Error(err)
				return
			}
			mu.Lock()
			proofs = append(proofs, proof)
			mu.Unlock()
		}(data)
	}
	wg.Wait()
	return proofs, nil
}

func (f *service) handleFraudMessageRequest(stream network.Stream) {
	msg := new(pb.FraudMessage)
	_, err := serde.Read(stream, msg)
	if err != nil {
		stream.Reset() //nolint:errcheck
		log.Warn(err)
		return
	}

	errg, ctx := errgroup.WithContext(context.TODO())
	mu := &sync.Mutex{}
	for _, p := range msg.RequestedProofType {
		proofType := toProof(p)
		errg.Go(func() error {
			f.topicsLk.Lock()
			t, ok := f.topics[proofType]
			f.topicsLk.RUnlock()
			if !ok {
				return fmt.Errorf("fraud: unmarshaler for proof type %s is not registered", proofType)
			}

			f.storesLk.RLock()
			store, ok := f.stores[proofType]
			f.storesLk.RUnlock()
			if !ok {
				return fmt.Errorf("fraud: no store for requested proofs type %s", proofType)
			}
			proofs, err := getAll(ctx, store, t.unmarshal)
			if err != nil {
				return err
			}

			for _, proof := range proofs {
				bin, err := proof.MarshalBinary()
				if err != nil {
					log.Warn(err)
					continue
				}
				resp := &pb.RespondedProof{Type: int32(proofType), Value: bin}
				mu.Lock()
				msg.RespondedProofs = append(msg.RespondedProofs, resp)
				mu.Unlock()
			}
			return nil
		})
	}

	if err = errg.Wait(); err != nil {
		stream.Reset() //nolint:errcheck
		log.Error(err)
		return
	}
	_, err = serde.Write(stream, msg)
	if err != nil {
		log.Error(err)
	}
}

func getSubTopic(p ProofType) string {
	return p.String() + "-sub"
}
