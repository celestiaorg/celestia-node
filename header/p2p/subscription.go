package p2p

import (
	"context"
	"fmt"
	"reflect"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/celestiaorg/celestia-node/header"
)

// subscription handles retrieving ExtendedHeaders from the header pubsub topic.
type subscription struct {
	topic        *pubsub.Topic
	subscription *pubsub.Subscription
}

// newSubscription creates a new ExtendedHeader event subscription
// on the given host.
func newSubscription(topic *pubsub.Topic) (*subscription, error) {
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	return &subscription{
		topic:        topic,
		subscription: sub,
	}, nil
}

// NextHeader returns the next (latest) verified ExtendedHeader from the network.
func (s *subscription) NextHeader(ctx context.Context) (*header.ExtendedHeader, error) {
	msg, err := s.subscription.Next(ctx)
	if err != nil {
		return nil, err
	}
	log.Debugw("received message", "topic", msg.Message.GetTopic(), "sender", msg.ReceivedFrom)

	header, ok := msg.ValidatorData.(*header.ExtendedHeader)
	if !ok {
		panic(fmt.Sprintf("invalid type received %s", reflect.TypeOf(msg.ValidatorData)))
	}

	log.Debugw("received new ExtendedHeader", "height", header.Height, "hash", header.Hash())
	return header, nil
}

// Cancel cancels the subscription to new ExtendedHeaders from the network.
func (s *subscription) Cancel() {
	s.subscription.Cancel()
}
