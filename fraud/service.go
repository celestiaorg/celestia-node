package fraud

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/celestiaorg/celestia-node/params"
)

// fraudRequests is the amount of external requests that will be tried to get fraud proofs from other peers.
const fraudRequests = 5

var fraudProtocolID = protocol.ID(fmt.Sprintf("/fraud/v0.0.1/%s", params.DefaultNetwork()))

// ProofService is responsible for validating and propagating Fraud Proofs.
// It implements the Service interface.
type ProofService struct {
	ctx    context.Context
	cancel context.CancelFunc

	topicsLk sync.RWMutex
	topics   map[ProofType]*pubsub.Topic

	storesLk sync.RWMutex
	stores   map[ProofType]datastore.Datastore

	pubsub *pubsub.PubSub
	host   host.Host
	getter headerFetcher
	ds     datastore.Datastore

	syncerEnabled bool
}

func NewProofService(
	p *pubsub.PubSub,
	host host.Host,
	getter headerFetcher,
	ds datastore.Datastore,
	syncerEnabled bool,
) *ProofService {
	return &ProofService{
		pubsub:        p,
		host:          host,
		getter:        getter,
		topics:        make(map[ProofType]*pubsub.Topic),
		stores:        make(map[ProofType]datastore.Datastore),
		ds:            ds,
		syncerEnabled: syncerEnabled,
	}
}

// RegisterProofs registers proofTypes as pubsub topics to be joined.
func (f *ProofService) RegisterProofs(proofTypes ...ProofType) error {
	for _, proofType := range proofTypes {
		t, err := join(f.pubsub, proofType, f.processIncoming)
		if err != nil {
			return err
		}
		f.topicsLk.Lock()
		f.topics[proofType] = t
		f.topicsLk.Unlock()
	}
	return nil
}

// Start sets the stream handler for fraudProtocolID and starts syncing if syncer is enabled.
func (f *ProofService) Start(context.Context) error {
	f.ctx, f.cancel = context.WithCancel(context.Background())
	f.host.SetStreamHandler(fraudProtocolID, f.handleFraudMessageRequest)
	if f.syncerEnabled {
		go f.syncFraudProofs(f.ctx)
	}
	return nil
}

// Stop removes the stream handler.
func (f *ProofService) Stop(context.Context) error {
	f.host.RemoveStreamHandler(fraudProtocolID)
	f.cancel()
	return nil
}

func (f *ProofService) Subscribe(proofType ProofType) (_ Subscription, err error) {
	f.topicsLk.Lock()
	defer f.topicsLk.Unlock()
	t, ok := f.topics[proofType]
	if !ok {
		return nil, fmt.Errorf("topic for %s does not exist", proofType)
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

// processIncoming encompasses the logic for validating fraud proofs.
func (f *ProofService) processIncoming(
	ctx context.Context,
	proofType ProofType,
	from peer.ID,
	msg *pubsub.Message,
) pubsub.ValidationResult {
	// unmarshal message to the Proof.
	// Peer will be added to black list if unmarshalling fails.
	proof, err := Unmarshal(proofType, msg.Data)
	if err != nil {
		log.Errorw("unmarshalling failed", "err", err)
		if !errors.Is(err, &errNoUnmarshaler{}) {
			f.pubsub.BlacklistPeer(from)
		}
		return pubsub.ValidationReject
	}
	// check the fraud proof locally and ignore if it has been already stored locally.
	if f.verifyLocal(ctx, proofType, hex.EncodeToString(proof.HeaderHash()), msg.Data) {
		return pubsub.ValidationIgnore
	}

	msg.ValidatorData = proof

	// fetch extended header in order to verify the fraud proof.
	extHeader, err := f.getter(ctx, proof.Height())
	if err != nil {
		log.Errorw("failed to fetch header to verify a fraud proof",
			"err", err, "proofType", proof.Type(), "height", proof.Height())
		return pubsub.ValidationIgnore
	}
	// validate the fraud proof.
	// Peer will be added to black list if the validation fails.
	err = proof.Validate(extHeader)
	if err != nil {
		log.Errorw("proof validation err: ",
			"err", err, "proofType", proof.Type(), "height", proof.Height())
		f.pubsub.BlacklistPeer(from)
		return pubsub.ValidationReject
	}

	// add the fraud proof to storage.
	err = f.put(ctx, proof.Type(), hex.EncodeToString(proof.HeaderHash()), msg.Data)
	if err != nil {
		log.Errorw("failed to store fraud proof", "err", err)
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

// put adds a fraud proof to the local storage.
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

// verifyLocal checks that fraud proofs has been stored locally.
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
