package fraud

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
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
	topics   map[ProofType]*topic

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
		topics: make(map[ProofType]*topic),
		stores: make(map[ProofType]datastore.Datastore),
		ds:     ds,
	}
}

func (f *service) Subscribe(proofType ProofType) (Subscription, error) {
	f.topicsLk.Lock()
	t, ok := f.topics[proofType]
	f.topicsLk.Unlock()
	if !ok {
		return nil, fmt.Errorf("fraud: unmarshaler for %s proof is not registered", proofType)
	}
	return newSubscription(t)
}

func (f *service) RegisterUnmarshaler(proofType ProofType, u ProofUnmarshaler) error {
	f.topicsLk.RLock()
	_, ok := f.topics[proofType]
	f.topicsLk.RUnlock()
	if ok {
		return fmt.Errorf("fraud: unmarshaler for %s proof is registered", proofType)
	}

	t, err := f.pubsub.Join(getSubTopic(proofType))
	if err != nil {
		return err
	}
	err = f.pubsub.RegisterTopicValidator(
		getSubTopic(proofType),
		func(ctx context.Context, _ peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
			return f.processIncoming(ctx, proofType, msg)
		},
	)
	if err != nil {
		return err
	}
	f.topicsLk.Lock()
	f.topics[proofType] = &topic{topic: t, unmarshal: u}
	f.topicsLk.Unlock()
	f.initStore(proofType)
	return nil
}

func (f *service) UnregisterUnmarshaler(proofType ProofType) error {
	f.topicsLk.Lock()
	defer f.topicsLk.Unlock()
	t, ok := f.topics[proofType]
	if !ok {
		return fmt.Errorf("fraud: unmarshaler for %s proof is not registered", proofType)
	}
	delete(f.topics, proofType)
	return t.close()

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
	return t.publish(ctx, bin)
}

func (f *service) processIncoming(
	ctx context.Context,
	proofType ProofType,
	msg *pubsub.Message,
) pubsub.ValidationResult {
	f.topicsLk.RLock()
	t, ok := f.topics[proofType]
	f.topicsLk.RUnlock()
	if !ok {
		panic("fraud: unmarshaler for the given proof type is not registered")
	}
	proof, err := t.unmarshal(msg.Data)
	if err != nil {
		log.Errorw("failed to unmarshal fraud proof", err)
		f.pubsub.BlacklistPeer(msg.ReceivedFrom)
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
		f.pubsub.BlacklistPeer(msg.ReceivedFrom)
		return pubsub.ValidationReject
	}
	log.Warnw("received fraud proof", "proofType", proof.Type(),
		"height", proof.Height(),
		"hash", hex.EncodeToString(extHeader.DAH.Hash()),
		"from", msg.ReceivedFrom.String(),
	)
	f.storesLk.RLock()
	store, ok := f.stores[proofType]
	f.storesLk.RUnlock()
	if ok {
		err = put(ctx, store, string(proof.HeaderHash()), msg.Data)
		if err != nil {
			log.Error(err)
		}
	} else {
		log.Warnf("no store for incoming proofs type %s", proof.Type())
	}
	log.Warn("Shutting down services...")
	return pubsub.ValidationAccept
}

func (f *service) Get(ctx context.Context, proofType ProofType) ([]Proof, error) {
	f.storesLk.RLock()
	store, ok := f.stores[proofType]
	f.storesLk.RUnlock()
	if !ok {
		return nil, fmt.Errorf("fraud: proof type %s is not supported", proofType)
	}

	f.topicsLk.RLock()
	t, ok := f.topics[proofType]
	f.topicsLk.RUnlock()
	if !ok {
		return nil, fmt.Errorf("fraud: unmarshaler for proof type %s is not registered", proofType)
	}

	return getAll(ctx, store, t.unmarshal)
}

func (f *service) initStore(proofType ProofType) {
	f.storesLk.Lock()
	defer f.storesLk.Unlock()
	_, ok := f.stores[proofType]
	if !ok {
		f.stores[proofType] = namespace.Wrap(f.ds, makeKey(proofType))
	}
}

func getSubTopic(p ProofType) string {
	return p.String() + "-sub"
}
