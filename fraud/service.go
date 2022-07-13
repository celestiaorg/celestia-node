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

	storeLk sync.RWMutex
	stores  map[ProofType]datastore.Datastore

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

func (f *service) ListenFraudProofs(
	ctx context.Context,
	proofType ProofType,
	start func(context.Context) error,
	stop func(context.Context) error,
) error {
	_, err := f.checkProofs(ctx, proofType)
	switch err {
	// this error is expected in most cases. It is thrown
	// when no Fraud Proof is stored
	case datastore.ErrNotFound:
		if err = start(ctx); err == nil {
			go onFraudProof(ctx, f, proofType, stop)
		}
	case nil:
		// TODO(@vgonkivs) validate all received Fraud Proofs
		return errors.New("fraud: fraud proof is stored. service will not be started")
	}
	return err
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
	f.removeStore(proofType)
	return t.close()

}

func (f *service) Broadcast(ctx context.Context, p Proof, opts ...pubsub.PubOpt) error {
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
	return t.publish(ctx, bin, opts...)
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

	// create a timeout for block fetching since our validator will be called synchronously
	// and getter is a blocking function.
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
	log.Warn("Shutting down services...")
	f.storeLk.RLock()
	store := f.stores[proofType]
	f.storeLk.RUnlock()
	if err := put(ctx, store, string(proof.HeaderHash()), msg.Data); err != nil {
		log.Warn(err)
	}

	return pubsub.ValidationAccept
}

func (f *service) GetAll(ctx context.Context, proofType ProofType) ([]Proof, error) {
	f.storeLk.Lock()
	store, ok := f.stores[proofType]
	if !ok {
		return nil, errors.New("fraud: fraud proof is not supported")
	}
	f.storeLk.Unlock()
	f.topicsLk.Lock()
	t, ok := f.topics[proofType]
	f.topicsLk.Unlock()
	if !ok {
		return nil, errors.New("fraud: unmarshaler for the given proof type is not registered")
	}

	return getAll(ctx, store, t.unmarshal)
}

func (f *service) initStore(proofType ProofType) {
	f.storeLk.Lock()
	defer f.storeLk.Unlock()
	_, ok := f.stores[proofType]
	if !ok {
		store := namespace.Wrap(f.ds, makeKey(proofType))
		f.stores[proofType] = store
	}
}

func (f *service) removeStore(proofType ProofType) {
	f.storeLk.Lock()

	store, ok := f.stores[proofType]
	if ok {
		err := store.Delete(context.TODO(), makeKey(proofType))
		if err != nil {
			log.Warn(err)
		}
		delete(f.stores, proofType)
	}
}

func (f *service) checkProofs(ctx context.Context, proofType ProofType) ([]Proof, error) {
	f.storeLk.Lock()
	store, ok := f.stores[proofType]
	if !ok {
		return nil, errors.New("fraud: fraud proof is not supported")
	}
	f.storeLk.Unlock()
	f.topicsLk.Lock()
	t, ok := f.topics[proofType]
	f.topicsLk.Unlock()
	if !ok {
		return nil, errors.New("fraud: unmarshaler for the given proof type is not registered")
	}
	return getAll(ctx, store, t.unmarshal)
}

func getSubTopic(p ProofType) string {
	return p.String() + "-sub"
}
