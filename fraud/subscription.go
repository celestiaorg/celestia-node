package fraud

import (
	"context"
	"errors"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// subscription wraps pubsub subscription and handle Fraud Proof from the pubsub topic.
type subscription struct {
	subscription *pubsub.Subscription
}

func (s *subscription) Proof(ctx context.Context) (Proof, error) {
	if s.subscription == nil {
		return nil, errors.New("fraud: subscription is not created")
	}
	data, err := s.subscription.Next(ctx)
	if err != nil {
		return nil, err
	}
	topic := s.subscription.Topic()
	unmarshaler, err := GetUnmarshaler(getProofTypeFromTopic(topic))
	if err != nil {
		return nil, err
	}
	return unmarshaler(data.Data)
}

func (s *subscription) Cancel() {
	s.subscription.Cancel()
}
