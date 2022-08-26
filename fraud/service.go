package fraud

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

const (
	// fetchHeaderTimeout duration of GetByHeight request to fetch an ExtendedHeader.
	fetchHeaderTimeout = time.Minute * 2
)

// service is responsible for validating and propagating Fraud Proofs.
// It implements the Service interface.
type service struct {
	topicsLk sync.RWMutex
	topics   map[ProofType]*pubsub.Topic

	storesLk sync.RWMutex
	stores   map[ProofType]datastore.Datastore

	pubsub *pubsub.PubSub
	getter headerFetcher
	ds     datastore.Datastore
}

func NewService(p *pubsub.PubSub, getter headerFetcher, ds datastore.Datastore) Service {
	return &service{
		pubsub: p,
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
