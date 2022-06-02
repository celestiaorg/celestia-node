package fraud

import (
	"context"
	"errors"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

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
		log.Debugf("successfully subscibed to topic: %s", getSubTopic(proofType))
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

	return errors.New("fraud: topic is not found")
}

// registerValidator adds an internal validation to topic inside libp2p for provided ProofType
func (t *topics) registerValidator(
	proofType ProofType,
	val func(context.Context, ProofType, []byte) pubsub.ValidationResult,
) error {
	return t.pubsub.RegisterTopicValidator(
		getSubTopic(proofType),
		func(ctx context.Context, _ peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
			return val(ctx, proofType, msg.Data)
		},
	)
}
