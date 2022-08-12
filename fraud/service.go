package fraud

import (
	"bytes"
	"context"
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

// ProofService is responsible for validating and propagating Fraud Proofs.
// It implements the Service interface.
type ProofService struct {
	topicsLk sync.RWMutex
	topics   map[ProofType]*pubsub.Topic

	storesLk sync.RWMutex
	stores   map[ProofType]datastore.Datastore

	pubsub *pubsub.PubSub
	host   host.Host
	getter headerFetcher
	ds     datastore.Datastore

	enabledSyncer bool
}

func NewService(
	p *pubsub.PubSub,
	host host.Host,
	getter headerFetcher,
	ds datastore.Datastore,
	enabledSyncer bool,
) *ProofService {
	return &ProofService{
		pubsub:        p,
		host:          host,
		getter:        getter,
		topics:        make(map[ProofType]*pubsub.Topic),
		stores:        make(map[ProofType]datastore.Datastore),
		ds:            ds,
		enabledSyncer: enabledSyncer,
	}
}

func (f *ProofService) Start(ctx context.Context) error {
	f.host.SetStreamHandler(fraudProtocolID, f.handleFraudMessageRequest)
	if f.enabledSyncer {
		go f.syncFraudProofs(ctx)
	}
	return nil
}

func (f *ProofService) Subscribe(proofType ProofType) (_ Subscription, err error) {
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

func (f *ProofService) Broadcast(ctx context.Context, p Proof) error {
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

func (f *ProofService) processIncoming(
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
	// we can skip incoming proof if it has been already stored locally
	if f.verifyLocal(ctx, proofType, proof.HeaderHash(), msg.Data) {
		return pubsub.ValidationIgnore
	}

	msg.ValidatorData = proof

	extHeader, err := f.getter(ctx, proof.Height())
	if err != nil {
		// TODO @vgonkivs: add retry mechanism to fetch header
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

	err = f.put(ctx, proof.Type(), proof.HeaderHash(), msg.Data)
	if err != nil {
		log.Error("error while adding a fraud proof to storage: ", err)
	}

	return pubsub.ValidationAccept
}

func (f *ProofService) Get(ctx context.Context, proofType ProofType) ([]Proof, error) {
	f.storesLk.Lock()
	store, ok := f.stores[proofType]
	if !ok {
		store = initStore(proofType, f.ds)
		f.stores[proofType] = store
	}
	f.storesLk.Unlock()

	return getAll(ctx, store, proofType)
}

func (f *ProofService) put(ctx context.Context, proofType ProofType, hash string, data []byte) error {
	f.storesLk.Lock()
	store, ok := f.stores[proofType]
	if !ok {
		store = initStore(proofType, f.ds)
		f.stores[proofType] = store
	}
	f.storesLk.Unlock()
	return put(ctx, store, hash, data)
}

func (f *ProofService) handleFraudMessageRequest(stream network.Stream) {
	req := &pb.FraudMessageRequest{}
	_, err := serde.Read(stream, req)
	if err != nil {
		stream.Reset() //nolint:errcheck
		log.Warnf("error while handling fraud message request: %s ", err)
		return
	}

	resp := &pb.FraudMessageResponse{}
	errg, ctx := errgroup.WithContext(context.Background())
	mu := sync.Mutex{}
	for _, p := range req.RequestedProofType {
		proofType := pbToProof(p)
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
				proof := &pb.ProofResponse{Type: proofToPb(proofType), Value: bin}
				mu.Lock()
				resp.Proofs = append(resp.Proofs, proof)
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
	_, err = serde.Write(stream, resp)
	if err != nil {
		log.Error("error while writing a response: %s", err)
	}
}

func (f *ProofService) syncFraudProofs(ctx context.Context) {
	log.Debug("start fetching Fraud Proofs")
	sub, err := f.host.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	if err != nil {
		log.Error(err)
		return
	}
	defer sub.Close()
	f.topicsLk.RLock()
	proofTypes := make([]pb.ProofType, 0, len(f.topics))
	for proofType := range f.topics {
		proofTypes = append(proofTypes, proofToPb(proofType))
	}
	f.topicsLk.RUnlock()
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
			log.Debug("requesting proofs from the peer ", pid)
			respProofs, err := requestProofs(ctx, f.host, pid, proofTypes)
			if err != nil {
				log.Errorw("error while requesting fraud proofs from the peer", "err", err, "peer", pid)
				return
			}
			if len(respProofs) == 0 {
				log.Debug("proofs were not found")
				return
			}
			log.Debug("got fraud proofs from the peer: ", connStatus.Peer)
			wg := sync.WaitGroup{}
			wg.Add(len(respProofs))
			for _, data := range respProofs {
				f.topicsLk.RLock()
				topic, ok := f.topics[pbToProof(data.Type)]
				f.topicsLk.RUnlock()
				if !ok {
					log.Errorf("topic for %s is not exist", pbToProof(data.Type))
					continue
				}
				err = topic.Publish(
					ctx,
					data.Value,
					// broadcast across all local subscriptions in order to verify fraud proof and to stop services
					pubsub.WithLocalPublication(true),
					// key can be nil because it will not be verified in this case
					pubsub.WithSecretKeyAndPeerId(nil, pid),
				)
				if err != nil {
					log.Error(err)
				}
			}
			wg.Wait()
		}(connStatus.Peer)
	}
}

func (f *ProofService) verifyLocal(ctx context.Context, proofType ProofType, hash string, data []byte) bool {
	f.storesLk.RLock()
	storage, ok := f.stores[proofType]
	f.storesLk.RUnlock()
	if !ok {
		return false
	}

	proof, err := getByHash(ctx, storage, hash)
	if err != nil {
		if !errors.Is(err, datastore.ErrNotFound) {
			log.Warn(err)
		}
		return false
	}

	return bytes.Equal(proof, data)
}
