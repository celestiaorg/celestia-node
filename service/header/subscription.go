package header

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
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
func (s *subscription) NextHeader(ctx context.Context) (*ExtendedHeader, error) {
	msg, err := s.subscription.Next(ctx)
	if err != nil {
		return nil, err
	}
	log.Debugw("received message", "topic", msg.Message.GetTopic(), "sender", msg.ReceivedFrom)

	var header ExtendedHeader
	err = header.UnmarshalBinary(msg.Data)
	if err != nil {
		log.Errorw("unmarshalling data from message", "err", err)
		return nil, err
	}

	log.Debugw("received new ExtendedHeader", "height", header.Height, "hash", header.Hash())
	return &header, nil
}

// Cancel cancels the subscription to new ExtendedHeaders from the network.
func (s *subscription) Cancel() {
	s.subscription.Cancel()
}
