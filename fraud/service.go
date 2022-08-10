package fraud

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	pb "github.com/celestiaorg/celestia-node/fraud/pb"
	"github.com/celestiaorg/celestia-node/params"
)

// fraudRequests is amount of external requests that will be done in order to get fraud proofs from another peers.
const fraudRequests = 5

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

func NewService(
	p *pubsub.PubSub,
	host host.Host,
	getter headerFetcher,
	ds datastore.Datastore,
) Service {
	s := &service{
		pubsub: p,
		host:   host,
		getter: getter,
		topics: make(map[ProofType]*pubsub.Topic),
		stores: make(map[ProofType]datastore.Datastore),
		ds:     ds,
	}

	host.SetStreamHandler(fraudProtocolID, s.handleFraudMessageRequest)
	return s
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
	msg.ValidatorData = proof
	if msg.Local {
		return f.processLocal(ctx, proofType, proof.HeaderHash(), msg.Data)
	}
	result := f.processExternal(ctx, proof, from)
	if result != pubsub.ValidationAccept {
		return result
	}

	err = f.put(ctx, proof.Type(), proof.HeaderHash(), msg.Data)
	if err != nil {
		log.Error(err)
	}
	return result
}

func (f *service) processExternal(
	ctx context.Context,
	proof Proof,
	from peer.ID,
) pubsub.ValidationResult {
	extHeader, err := f.getter(ctx, proof.Height())
	if err != nil {
		log.Errorw("failed to fetch header to verify a fraud proof",
			"err", err, "proofType", proof.Type(), "height", proof.Height())
		return pubsub.ValidationReject
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
	log.Warn("Shutting down services...")
	return pubsub.ValidationAccept
}

func (f *service) processLocal(
	ctx context.Context,
	proofType ProofType,
	hash string,
	rawData []byte,
) pubsub.ValidationResult {
	f.storesLk.RLock()
	storage := f.stores[proofType]
	f.storesLk.RUnlock()
	data, err := getByHash(ctx, storage, hash)
	if err != nil {
		// TODO @vgonkivs: should we panic here???
		panic(fmt.Sprintf("unexpected error while fetching a proof from the storage: %s", err.Error()))
	}

	if bytes.Compare(data, rawData) == 0 {
		log.Warnw("received fraud proof",
			"proofType", proofType,
			"headerHash", hash,
		)
		return pubsub.ValidationAccept
	}
	return pubsub.ValidationReject
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

func (f *service) put(ctx context.Context, proofType ProofType, hash string, data []byte) error {
	f.storesLk.Lock()
	store, ok := f.stores[proofType]
	if !ok {
		store = initStore(proofType, f.ds)
		f.stores[proofType] = store
	}
	f.storesLk.Unlock()
	return put(ctx, store, hash, data)
}

func (f *service) handleFraudMessageRequest(stream network.Stream) {
	msg := new(pb.FraudMessage)
	_, err := serde.Read(stream, msg)
	if err != nil {
		stream.Reset() //nolint:errcheck
		log.Warn(err)
		return
	}

	errg, ctx := errgroup.WithContext(context.Background())
	mu := sync.Mutex{}
	for _, p := range msg.RequestedProofType {
		proofType := toProof(p)
		errg.Go(func() error {
			proofs, err := f.Get(ctx, proofType)
			if err != nil {
				if errors.Is(err, datastore.ErrNotFound) {
					return nil
				}
				return err
			}

			for _, proof := range proofs {
				bin, err := proof.MarshalBinary()
				if err != nil {
					log.Warn(err)
					continue
				}
				resp := &pb.ProofResponse{Type: int32(proofType), Value: bin}
				mu.Lock()
				msg.Proofs = append(msg.Proofs, resp)
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

func (f *service) Sync(ctx context.Context) {
	go f.syncFraudProofs(ctx)
}

func (f *service) syncFraudProofs(ctx context.Context) {
	log.Debug("start fetching Fraud Proofs")
	sub, err := f.host.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	if err != nil {
		log.Error(err)
		return
	}
	defer sub.Close()
	connStatus := event.EvtPeerIdentificationCompleted{}
	for i := 0; i < fraudRequests; i++ {
		select {
		case <-ctx.Done():
			return
		case e := <-sub.Out():
			connStatus = e.(event.EvtPeerIdentificationCompleted)
		}

		if connStatus.Peer == f.host.ID() {
			continue
		}
		go func(pid peer.ID) {
			log.Debug("requesting proofs from peer ", pid)
			f.topicsLk.RLock()
			proofTypes := make([]int32, 0, len(f.topics))
			for proofType := range f.topics {
				proofTypes = append(proofTypes, int32(proofType))
			}
			f.topicsLk.RUnlock()
			respProofs, err := requestProofs(ctx, f.host, pid, proofTypes)
			if err != nil {
				log.Error(err)
				return
			}
			if len(respProofs) == 0 {
				log.Debug("proof was not found")
				return
			}
			log.Debug("got fraud proofs from peer: ", connStatus.Peer)
			wg := sync.WaitGroup{}
			wg.Add(len(respProofs))
			for _, data := range respProofs {
				go func(data *pb.ProofResponse) {
					defer wg.Done()
					proof, err := Unmarshal(toProof(data.Type), data.Value)
					if err != nil {
						if !errors.Is(err, &errNoUnmarshaler{}) {
							f.pubsub.BlacklistPeer(pid)
						}
						return
					}
					h, err := f.getter(ctx, proof.Height())
					if err != nil {
						log.Error(err)
						return
					}
					err = proof.Validate(h)
					if err != nil {
						f.pubsub.BlacklistPeer(pid)
						return
					}

					err = f.put(ctx, proof.Type(), proof.HeaderHash(), data.Value)
					if err != nil {
						log.Error(err)
					}

					f.topicsLk.RLock()
					topic, ok := f.topics[proof.Type()]
					f.topicsLk.RUnlock()
					if !ok {
						log.Error("topic is not registered for proof ", proof.Type())
						return
					}
					err = topic.Publish(ctx, data.Value, pubsub.WithLocalPublication(true))
					if err != nil {
						log.Error(err)
					}
				}(data)
			}
			wg.Wait()
		}(connStatus.Peer)
	}
}
