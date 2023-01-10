package p2p

import (
	"context"
	"fmt"
	"reflect"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/celestiaorg/celestia-node/libs/header"
)

// subscription handles retrieving Headers from the header pubsub topic.
type subscription[H header.Header] struct {
	topic        *pubsub.Topic
	subscription *pubsub.Subscription
}

// newSubscription creates a new Header event subscription
// on the given host.
func newSubscription[H header.Header](topic *pubsub.Topic) (*subscription[H], error) {
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	return &subscription[H]{
		topic:        topic,
		subscription: sub,
	}, nil
}

// NextHeader returns the next (latest) verified Header from the network.
func (s *subscription[H]) NextHeader(ctx context.Context) (H, error) {
	msg, err := s.subscription.Next(ctx)
	if err != nil {
		var zero H
		return zero, err
	}
	log.Debugw("received message", "topic", msg.Message.GetTopic(), "sender", msg.ReceivedFrom)

	header, ok := msg.ValidatorData.(H)
	if !ok {
		panic(fmt.Sprintf("invalid type received %s", reflect.TypeOf(msg.ValidatorData)))
	}

	log.Debugw("received new Header", "height", header.Height(), "hash", header.Hash())
	return header, nil
}

// Cancel cancels the subscription to new Headers from the network.
func (s *subscription[H]) Cancel() {
	s.subscription.Cancel()
}
