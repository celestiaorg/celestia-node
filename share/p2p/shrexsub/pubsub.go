package shrexsub

import (
	"context"
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/share"
	pb "github.com/celestiaorg/celestia-node/share/p2p/shrexsub/pb"
)

var log = logging.Logger("shrex-sub")

// pubsubTopic hardcodes the name of the EDS floodsub topic with the provided networkID.
func pubsubTopicID(networkID string) string {
	return fmt.Sprintf("%s/eds-sub/v0.1.0", networkID)
}

// ValidatorFn is an injectable func and governs EDS notification msg validity.
// It receives the notification and sender peer and expects the validation result.
// ValidatorFn is allowed to be blocking for an indefinite time or until the context is canceled.
type ValidatorFn func(context.Context, peer.ID, Notification) pubsub.ValidationResult

// BroadcastFn aliases the function that broadcasts the DataHash.
type BroadcastFn func(context.Context, Notification) error

// Notification is the format of message sent by Broadcaster
type Notification struct {
	DataHash share.DataHash
	Height   uint64
}

// PubSub manages receiving and propagating the EDS from/to the network
// over "eds-sub" subscription.
type PubSub struct {
	pubSub *pubsub.PubSub
	topic  *pubsub.Topic

	pubsubTopic string
	cancelRelay pubsub.RelayCancelFunc
}

// NewPubSub creates a libp2p.PubSub wrapper.
func NewPubSub(ctx context.Context, h host.Host, networkID string) (*PubSub, error) {
	pubsub, err := pubsub.NewFloodSub(ctx, h)
	if err != nil {
		return nil, err
	}
	return &PubSub{
		pubSub:      pubsub,
		pubsubTopic: pubsubTopicID(networkID),
	}, nil
}

// Start creates an instances of FloodSub and joins specified topic.
func (s *PubSub) Start(context.Context) error {
	topic, err := s.pubSub.Join(s.pubsubTopic)
	if err != nil {
		return err
	}

	cancel, err := topic.Relay()
	if err != nil {
		return err
	}

	s.cancelRelay = cancel
	s.topic = topic
	return nil
}

// Stop completely stops the PubSub:
// * Unregisters all the added Validators
// * Closes the `ShrEx/Sub` topic
func (s *PubSub) Stop(context.Context) error {
	s.cancelRelay()
	err := s.pubSub.UnregisterTopicValidator(s.pubsubTopic)
	if err != nil {
		log.Warnw("unregistering topic", "err", err)
	}
	return s.topic.Close()
}

// AddValidator registers given ValidatorFn for EDS notifications.
// Any amount of Validators can be registered.
func (s *PubSub) AddValidator(v ValidatorFn) error {
	return s.pubSub.RegisterTopicValidator(s.pubsubTopic, v.validate)
}

func (v ValidatorFn) validate(ctx context.Context, p peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
	var pbmsg pb.RecentEDSNotification
	if err := pbmsg.Unmarshal(msg.Data); err != nil {
		log.Debugw("validator: unmarshal error", "err", err)
		return pubsub.ValidationReject
	}

	n := Notification{
		DataHash: pbmsg.DataHash,
		Height:   pbmsg.Height,
	}
	if n.Height == 0 {
		// hard reject malicious height (height 0 does not exist)
		return pubsub.ValidationReject
	}
	if n.DataHash.Validate() != nil {
		// hard reject any data hashes that do not pass basic validation
		return pubsub.ValidationReject
	}
	if n.DataHash.IsEmptyRoot() {
		// we don't send empty EDS data hashes, but If someone sent it to us - do hard reject
		return pubsub.ValidationReject
	}
	return v(ctx, p, n)
}

// Subscribe provides a new Subscription for EDS notifications.
func (s *PubSub) Subscribe() (*Subscription, error) {
	if s.topic == nil {
		return nil, fmt.Errorf("shrex-sub: topic is not started")
	}
	return newSubscription(s.topic)
}

// Broadcast sends the EDS notification (DataHash) to every connected peer.
func (s *PubSub) Broadcast(ctx context.Context, notification Notification) error {
	if notification.DataHash.IsEmptyRoot() {
		// no need to broadcast datahash of an empty block EDS
		return nil
	}

	msg := pb.RecentEDSNotification{
		Height:   notification.Height,
		DataHash: notification.DataHash,
	}
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("shrex-sub: marshal notification, %w", err)
	}
	return s.topic.Publish(ctx, data)
}
