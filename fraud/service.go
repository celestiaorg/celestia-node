package fraud

import (
	"context"
	"encoding/hex"
	"errors"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// service is responsible for validating and propagating Fraud Proofs.
// It implements the Service interface.
type service struct {
	mu           sync.RWMutex
	unmarshalers map[ProofType]ProofUnmarshaler

	topics *topics
	getter headerFetcher
}

func NewService(p *pubsub.PubSub, getter headerFetcher) Service {
	return &service{
		topics: &topics{
			pubsub:       p,
			pubSubTopics: make(map[ProofType]*pubsub.Topic),
		},
		unmarshalers: make(map[ProofType]ProofUnmarshaler),
		getter:       getter,
	}
}

func (f *service) Subscribe(proofType ProofType) (Subscription, error) {
	// TODO: @vgonkivs check if fraud proof is in fraud store, then return with error
	f.mu.Lock()
	u, ok := f.unmarshalers[proofType]
	if !ok {
		return nil, errors.New("fraud: unmarshaler is not registered")
	}

	t, ok := f.topics.getTopic(proofType)

	// if topic was not stored in cache, then we should register a validator and
	// join pubsub topic.
	if !ok {
		err := f.topics.registerValidator(proofType, f.processIncoming)
		if err != nil {
			f.mu.Unlock()
			return nil, err
		}
		t, err = f.topics.join(proofType)
		if err != nil {
			f.mu.Unlock()
			return nil, err
		}
	}
	f.mu.Unlock()
	return newSubscription(t, u)
}

func (f *service) RegisterUnmarshaler(proofType ProofType, u ProofUnmarshaler) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.unmarshalers[proofType]; ok {
		return errors.New("fraud: unmarshaler is already registered")
	}
	f.unmarshalers[proofType] = u

	return nil
}

func (f *service) UnregisterUnmarshaler(proofType ProofType) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.unmarshalers[proofType]; !ok {
		return errors.New("fraud: unmarshaler is not registered")
	}
	delete(f.unmarshalers, proofType)

	return nil

}

func (f *service) Broadcast(ctx context.Context, p Proof) error {
	bin, err := p.MarshalBinary()
	if err != nil {
		return err
	}

	return f.topics.publish(ctx, bin, p.Type())
}

func (f *service) processIncoming(
	ctx context.Context,
	proofType ProofType,
	msg *pubsub.Message,
) pubsub.ValidationResult {
	f.mu.RLock()
	unmarshaler, ok := f.unmarshalers[proofType]
	f.mu.RUnlock()
	if !ok {
		log.Error("unmarshaler is not found")
		return pubsub.ValidationReject
	}
	proof, err := unmarshaler(msg.Data)
	if err != nil {
		log.Errorw("unmarshaling header error", err)
		return pubsub.ValidationReject
	}

	extHeader, err := f.getter(ctx, proof.Height())
	if err != nil {
		log.Errorw("failed to fetch block: ",
			"err", err)
		return pubsub.ValidationReject
	}

	err = proof.Validate(extHeader)
	if err != nil {
		log.Errorw("validation err: ",
			"err", err)
		f.topics.addPeerToBlacklist(msg.ReceivedFrom)
		return pubsub.ValidationReject
	}
	log.Warnw("received", "fp", proof.Type(),
		"hash", hex.EncodeToString(extHeader.DAH.Hash()),
		"from", msg.ReceivedFrom.String(),
	)
	return pubsub.ValidationAccept
}

func getSubTopic(p ProofType) string {
	return p.String() + "-sub"
}
