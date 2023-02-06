package p2p

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/libs/header"
)

// Subscriber manages the lifecycle and relationship of header Module
// with the "header-sub" gossipsub topic.
type Subscriber[H header.Header] struct {
	pubsubTopicID string

	pubsub *pubsub.PubSub
	topic  *pubsub.Topic
	msgID  pubsub.MsgIdFunction
}

// NewSubscriber returns a Subscriber that manages the header Module's
// relationship with the "header-sub" gossipsub topic.
func NewSubscriber[H header.Header](
	ps *pubsub.PubSub,
	msgID pubsub.MsgIdFunction,
	protocolSuffix string,
) *Subscriber[H] {
	return &Subscriber[H]{
		pubsubTopicID: pubsubTopicID(protocolSuffix),
		pubsub:        ps,
		msgID:         msgID,
	}
}

// Start starts the Subscriber, registering a topic validator for the "header-sub"
// topic and joining it.
func (p *Subscriber[H]) Start(context.Context) (err error) {
	log.Infow("joining topic", "topic ID", p.pubsubTopicID)
	p.topic, err = p.pubsub.Join(p.pubsubTopicID, pubsub.WithTopicMessageIdFn(p.msgID))
	return err
}

// Stop closes the topic and unregisters its validator.
func (p *Subscriber[H]) Stop(context.Context) error {
	err := p.pubsub.UnregisterTopicValidator(p.pubsubTopicID)
	if err != nil {
		log.Warnf("unregistering validator: %s", err)
	}

	return p.topic.Close()
}

// AddValidator applies basic pubsub validator for the topic.
func (p *Subscriber[H]) AddValidator(val func(context.Context, H) pubsub.ValidationResult) error {
	pval := func(ctx context.Context, p peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
		var empty H
		maybeHead := empty.New()
		err := maybeHead.UnmarshalBinary(msg.Data)
		if err != nil {
			log.Errorw("unmarshalling header",
				"from", p.ShortString(),
				"err", err)
			return pubsub.ValidationReject
		}
		msg.ValidatorData = maybeHead
		return val(ctx, maybeHead.(H))
	}
	return p.pubsub.RegisterTopicValidator(p.pubsubTopicID, pval)
}

// Subscribe returns a new subscription to the Subscriber's
// topic.
func (p *Subscriber[H]) Subscribe() (header.Subscription[H], error) {
	if p.topic == nil {
		return nil, fmt.Errorf("header topic is not instantiated, service must be started before subscribing")
	}

	return newSubscription[H](p.topic)
}

// Broadcast broadcasts the given Header to the topic.
func (p *Subscriber[H]) Broadcast(ctx context.Context, header H, opts ...pubsub.PubOpt) error {
	bin, err := header.MarshalBinary()
	if err != nil {
		return err
	}
	return p.topic.Publish(ctx, bin, opts...)
}
