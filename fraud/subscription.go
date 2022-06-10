package fraud

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// subscription handles Fraud Proof from the pubsub topic.
type subscription struct {
	subscription *pubsub.Subscription
	unmarshaler  ProofUnmarshaler
}

func newSubscription(t *topic) (*subscription, error) {
	sub, err := t.topic.Subscribe()
	if err != nil {
		return nil, err
	}

	return &subscription{sub, t.codec}, nil
}

func (s *subscription) Proof(ctx context.Context) (Proof, error) {
	data, err := s.subscription.Next(ctx)
	if err != nil {
		return nil, err
	}
	proof, err := s.unmarshaler(data.Data)
	if err != nil {
		return nil, err
	}
	return proof, nil
}

func (s *subscription) Cancel() {
	s.subscription.Cancel()
}
