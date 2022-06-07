package fraud

import (
	"context"
	"errors"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// topics allows to operate with pubsub connection and pubsub topics.
type topics struct {
	mu           sync.RWMutex
	pubSubTopics map[ProofType]*pubsub.Topic

	pubsub *pubsub.PubSub
}

// join allows to subscribe on pubsub topic
func (t *topics) join(
	proofType ProofType,
) (*pubsub.Topic, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	topic, err := t.pubsub.Join(getSubTopic(proofType))
	if err != nil {
		return nil, err
	}
	log.Debugf("successfully subscibed to topic: %s", getSubTopic(proofType))
	t.pubSubTopics[proofType] = topic
	return topic, nil
}

// getTopic returns pubsub topic.
func (t *topics) getTopic(proofType ProofType) (*pubsub.Topic, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	topic, ok := t.pubSubTopics[proofType]
	return topic, ok
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

// registerValidator adds an internal validation to topic inside libp2p for provided ProofType.
func (t *topics) registerValidator(
	proofType ProofType,
	val func(context.Context, ProofType, *pubsub.Message) pubsub.ValidationResult,
) error {
	return t.pubsub.RegisterTopicValidator(
		getSubTopic(proofType),
		func(ctx context.Context, _ peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
			return val(ctx, proofType, msg)
		},
	)
}

// addPeerToBlacklist adds a peer to pubsub blacklist to avoid receiving messages
// from it in the future.
func (t *topics) addPeerToBlacklist(peer peer.ID) {
	t.pubsub.BlacklistPeer(peer)
}
