package fraud

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/celestiaorg/celestia-node/header"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// headerFetcher aliases a function that is used to fetch ExtendedHeader from store
type headerFetcher func(context.Context, uint64) (*header.ExtendedHeader, error)

// topics wraps the map with ProofType and pubsub topic and RWMutex
type topics struct {
	t  map[ProofType]*pubsub.Topic
	mu sync.RWMutex
}

// service is propagating and validating Fraud Proofs
// service implements Service interface.
type service struct {
	pubsub       *pubsub.PubSub
	topics       *topics
	unmarshalers map[ProofType]ProofUnmarshaler
	getter       headerFetcher
	mu           sync.RWMutex
}

func NewService(p *pubsub.PubSub, getter headerFetcher) Service {
	return &service{
		pubsub: p,
		topics: &topics{
			t: make(map[ProofType]*pubsub.Topic),
		},
		unmarshalers: make(map[ProofType]ProofUnmarshaler),
		getter:       getter,
	}
}

func (f *service) Start(context.Context) error {
	log.Info("fraud service is starting...")

	// Add validator and default Unmarshaler for BEFP
	err := f.AddValidator(BadEncoding, f.processIncoming)
	if err != nil {
		return err
	}
	err = f.RegisterUnmarshaler(BadEncoding, UnmarshalBEFP)
	if err != nil {
		return err
	}
	return nil
}

func (f *service) Stop(context.Context) error {
	log.Info("fraud service is stopping...")
	return nil
}

func (f *service) Subscribe(proofType ProofType) (Subscription, error) {
	// TODO: @vgonkivs check if fraud proof is in fraud store, then return with error
	f.mu.RLock()
	u, ok := f.unmarshalers[proofType]
	if !ok {
		return nil, errors.New("unmarshaler is not registered")
	}
	f.mu.RUnlock()

	f.topics.mu.Lock()
	t, ok := f.topics.t[proofType]
	if !ok {
		var err error
		t, err = f.pubsub.Join(getSubTopic(proofType))
		if err != nil {
			return nil, err
		}
		log.Debugf("successfully subscibed to topic", t.String())
		f.topics.t[proofType] = t
	}
	f.topics.mu.Unlock()

	return newSubscription(t, u)
}

func (f *service) RegisterUnmarshaler(proofType ProofType, u ProofUnmarshaler) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if _, ok := f.unmarshalers[proofType]; ok {
		return errors.New("unmarshaler is registered")
	}
	f.unmarshalers[proofType] = u

	return nil
}

func (f *service) UnregisterUnmarshaler(proofType ProofType) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if _, ok := f.unmarshalers[proofType]; !ok {
		return errors.New("unmarshaler is not registered")
	}
	delete(f.unmarshalers, proofType)

	return nil

}

func (f *service) Broadcast(ctx context.Context, p Proof) error {
	bin, err := p.MarshalBinary()
	if err != nil {
		return err
	}

	f.topics.mu.RLock()
	defer f.topics.mu.RUnlock()

	if topic, ok := f.topics.t[p.Type()]; ok {
		return topic.Publish(ctx, bin)
	}

	return errors.New("topic is not found")
}

func (f *service) AddValidator(proofType ProofType, val Validator) error {
	topic := getSubTopic(proofType)
	pval := func(ctx context.Context, _ peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
		return val(ctx, proofType, msg.Data)
	}

	return f.pubsub.RegisterTopicValidator(topic, pval)
}

func (f *service) processIncoming(ctx context.Context, proofType ProofType, data []byte) pubsub.ValidationResult {
	f.mu.RLock()
	defer f.mu.RUnlock()
	unmarshaler, ok := f.unmarshalers[proofType]
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
	log.Debugw("received Bad Encoding Fraud Proof from block ", "hash", hex.EncodeToString(extHeader.DAH.Hash()))
	return pubsub.ValidationAccept
}

func getSubTopic(p ProofType) string {
	switch p {
	case BadEncoding:
		return "befp-sub"
	default:
		panic(fmt.Sprintf("invalid proof type %d", p))
	}
}
