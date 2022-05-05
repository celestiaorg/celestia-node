package fraud

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// subscription handles Fraud Proof from the pubsub topic.
type subscription struct {
	subscription *pubsub.Subscription
	unmarshaler  proofUnmarshaler
}

func newSubscription(topic *pubsub.Topic, u proofUnmarshaler) (*subscription, error) {
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	return &subscription{sub, u}, nil
}

func (s *subscription) Proof(ctx context.Context) (Proof, error) {
	data, err := s.subscription.Next(ctx)
	if err != nil {
		return nil, fmt.Errorf("error during listening to the next proof: %s ", err.Error())
	}
	proof, err := s.unmarshaler(data.Data)
	if err != nil {
		return nil, err
	}
	log.Debugw("fraud_proof", proof.Type(), " received at ", "height", proof.Height())
	return proof, nil
}

func (s *subscription) Cancel() {
	s.subscription.Cancel()
}
