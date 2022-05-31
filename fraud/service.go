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

// topics allows to operate with pubsub connection and pubsub topics
type topics struct {
	pubsub       *pubsub.PubSub
	pubSubTopics map[ProofType]*pubsub.Topic
	mu           sync.RWMutex
}

// getTopic joins a pubsub.Topic if it was not joined before and returns it
func (t *topics) getTopic(proofType ProofType) (*pubsub.Topic, bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	topic, ok := t.pubSubTopics[proofType]
	if !ok {
		var err error
		topic, err = t.pubsub.Join(getSubTopic(proofType))
		if err != nil {
			return nil, ok, err
		}
		log.Debugf("successfully subscibed to topic", getSubTopic(proofType))
		t.pubSubTopics[proofType] = topic
	}

	return topic, ok, nil
}

// publish allows to publish Fraud Proofs to the network
func (t *topics) publish(ctx context.Context, data []byte, proofType ProofType) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if topic, ok := t.pubSubTopics[proofType]; ok {
		return topic.Publish(ctx, data)
	}

	return errors.New("topic is not found")
}

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

func (f *service) Start(context.Context) error {
	log.Info("fraud service is starting...")

	err := f.RegisterUnmarshaler(BadEncoding, UnmarshalBEFP)
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
	f.mu.RUnlock()
	if !ok {
		return nil, errors.New("unmarshaler is not registered")
	}

	t, wasjoined, err := f.topics.getTopic(proofType)
	if err != nil {
		return nil, err
	}
	// if topic was joined for the first time then we should register a validator for it
	if !wasjoined {
		// add internal validation to topic inside libp2p for provided ProofType
		if err = f.topics.pubsub.RegisterTopicValidator(
			getSubTopic(proofType),
			func(ctx context.Context, _ peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
				return f.processIncoming(ctx, proofType, msg.Data)
			},
		); err != nil {
			return nil, err
		}
	}

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

	return f.topics.publish(ctx, bin, p.Type())
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
