package shrexsub

import (
	"context"
	"fmt"

	logging "github.com/ipfs/go-log/v2"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/share"
)

var log = logging.Logger("shrex-sub")

// pubSubTopic hardcodes the name of the EDS floodsub topic with the provided suffix.
func pubSubTopic(suffix string) string {
	return "eds-sub/v0.0.1/" + suffix
}

// Validator is an injectable func and governs EDS notification or DataHash validity.
// It receives the notification and sender peer and expects the validation result.
// Validator is allowed to be blocking for an indefinite time or until the context is canceled.
type Validator func(context.Context, peer.ID, share.DataHash) pubsub.ValidationResult

// BroadcastFn aliases the function that broadcasts the DataHash.
type BroadcastFn func(context.Context, share.DataHash) error

// PubSub manages receiving and propagating the EDS from/to the network
// over "eds-sub" subscription.
type PubSub struct {
	pubSub *pubsub.PubSub
	topic  *pubsub.Topic

	pubSubTopic string
}

// NewPubSub creates a libp2p.PubSub wrapper.
func NewPubSub(ctx context.Context, h host.Host, suffix string) (*PubSub, error) {
	// WithSeenMessagesTTL without duration allows to process all incoming messages(even with the same msgId)
	pubsub, err := pubsub.NewFloodSub(ctx, h, pubsub.WithSeenMessagesTTL(0))
	if err != nil {
		return nil, err
	}
	return &PubSub{
		pubSub:      pubsub,
		pubSubTopic: pubSubTopic(suffix),
	}, nil
}

// Start creates an instances of FloodSub and joins specified topic.
func (s *PubSub) Start(context.Context) error {
	topic, err := s.pubSub.Join(s.pubSubTopic)
	if err != nil {
		return err
	}

	s.topic = topic
	return nil
}

// Stop completely stops the PubSub:
// * Unregisters all the added Validators
// * Closes the `ShrEx/Sub` topic
func (s *PubSub) Stop(context.Context) error {
	err := s.pubSub.UnregisterTopicValidator(s.pubSubTopic)
	if err != nil {
		log.Warnw("unregistering topic", "err", err)
	}
	return s.topic.Close()
}

// AddValidator registers given Validator for EDS notifications (DataHash).
// Any amount of Validators can be registered.
func (s *PubSub) AddValidator(validate Validator) error {
	return s.pubSub.RegisterTopicValidator(s.pubSubTopic,
		func(ctx context.Context, p peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
			return validate(ctx, p, msg.Data)
		})
}

// Subscribe provides a new Subscription for EDS notifications.
func (s *PubSub) Subscribe() (*Subscription, error) {
	if s.topic == nil {
		return nil, fmt.Errorf("shrex-push: topic is not started")
	}
	return newSubscription(s.topic)
}

// Broadcast sends the EDS notification (DataHash) to every connected peer.
func (s *PubSub) Broadcast(ctx context.Context, data share.DataHash) error {
	return s.topic.Publish(ctx, data)
}
