package header

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// PubSubTopic hardcodes the name of the ExtendedHeader
// gossipsub topic.
const PubSubTopic = "header-sub"

// P2PSubscriber manages the lifecycle and relationship of header Service
// with the "header-sub" gossipsub topic.
type P2PSubscriber struct {
	pubsub *pubsub.PubSub
	topic  *pubsub.Topic
}

// NewP2PSubscriber returns a P2PSubscriber that manages the header Service's
// relationship with the "header-sub" gossipsub topic.
func NewP2PSubscriber(ps *pubsub.PubSub) *P2PSubscriber {
	return &P2PSubscriber{
		pubsub:    ps,
	}
}

// Start starts the P2PSubscriber, registering a topic validator for the "header-sub"
// topic and joining it.
func (p *P2PSubscriber) Start(context.Context) (err error) {
	p.topic, err = p.pubsub.Join(PubSubTopic)
	return err
}

// Stop closes the topic and unregisters its validator.
func (p *P2PSubscriber) Stop(context.Context) error {
	err := p.pubsub.UnregisterTopicValidator(PubSubTopic)
	if err != nil {
		return err
	}

	return p.topic.Close()
}

// AddValidator applies basic pubsub validator for a topic.
// Multiple validators can be registered.
func (p *P2PSubscriber) AddValidator(val pubsub.ValidatorEx) error {
	return p.pubsub.RegisterTopicValidator(PubSubTopic, val)
}

// Subscribe returns a new subscription to the P2PSubscriber's
// topic.
func (p *P2PSubscriber) Subscribe() (Subscription, error) {
	if p.topic == nil {
		return nil, fmt.Errorf("header topic is not instantiated, service must be started before subscribing")
	}

	return newSubscription(p.topic)
}

// Broadcast broadcasts the given ExtendedHeader to the topic.
func (p *P2PSubscriber) Broadcast(ctx context.Context, header *ExtendedHeader) error {
	bin, err := header.MarshalBinary()
	if err != nil {
		return err
	}
	return p.topic.Publish(ctx, bin)
}
