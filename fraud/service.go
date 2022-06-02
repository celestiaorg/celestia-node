package fraud

import (
	"context"
	"encoding/hex"
	"errors"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// service is propagating and validating Fraud Proofs
// service implements Service interface.
type service struct {
	topics       *topics
	unmarshalers map[ProofType]ProofUnmarshaler
	getter       headerFetcher
	mu           sync.RWMutex
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
	f.mu.RLock()
	u, ok := f.unmarshalers[proofType]
	f.mu.RUnlock()
	if !ok {
		return nil, errors.New("fraud: unmarshaler is not registered")
	}

	t, wasjoined, err := f.topics.getTopic(proofType)
	if err != nil {
		return nil, err
	}
	// if topic was joined for the first time then we should register a validator for it
	if !wasjoined {
		if err = f.topics.registerValidator(proofType, f.processIncoming); err != nil {
			return nil, err
		}
	}

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

func (f *service) processIncoming(ctx context.Context, proofType ProofType, data []byte) pubsub.ValidationResult {
	f.mu.RLock()
	unmarshaler, ok := f.unmarshalers[proofType]
	f.mu.RUnlock()
	if !ok {
		log.Error("unmarshaler is not found")
		return pubsub.ValidationReject
	}
	proof, err := unmarshaler(data)
	if err != nil {
		log.Errorw("unmarshalling header error", err)
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
		return pubsub.ValidationReject
	}
	log.Warnw("received Bad Encoding Fraud Proof from block ", "hash", hex.EncodeToString(extHeader.DAH.Hash()))
	return pubsub.ValidationAccept
}

func getSubTopic(p ProofType) string {
	return p.String() + "-sub"
}
