package fraud

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// subscription handles Fraud Proof from the pubsub topic.
type subscription struct {
	topic        *pubsub.Topic
	subscription *pubsub.Subscription
	unmarshaller proofUnmarshaller
}

func newSubscription(topic *pubsub.Topic, u proofUnmarshaller) (*subscription, error) {
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	return &subscription{topic, sub, u}, nil
}

func (s *subscription) Proof(ctx context.Context) (Proof, error) {
	data, err := s.subscription.Next(ctx)
	if err != nil {
		return nil, fmt.Errorf("error during listening to the next proof: %s ", err.Error())
	}

	return s.unmarshaller(data.Data)
}

func (s *subscription) Cancel() {
	s.subscription.Cancel()
}
