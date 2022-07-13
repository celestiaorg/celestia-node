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

	return &subscription{sub, t.unmarshal}, nil
}

func (s *subscription) Proof(ctx context.Context) (Proof, error) {
	data, err := s.subscription.Next(ctx)
	if err != nil {
		return nil, err
	}

	return s.unmarshaler(data.Data)
}

func (s *subscription) Cancel() {
	s.subscription.Cancel()
}

// OnProof accepts the subscription and waiting for the next Fraud Proof.
// In case if Fraud Proof is received, then handleF will be invoked.
func OnProof(ctx context.Context, subscription Subscription, handle func(ctx context.Context) error) {
	var err error
	defer func() {
		subscription.Cancel()
		if err == context.Canceled {
			return
		}
		stopErr := handle(ctx)
		if stopErr != nil {
			log.Warn(stopErr)
		}
	}()
	// At this point we receive already verified fraud proof,
	// so there are no needs to call Validate.
	_, err = subscription.Proof(ctx)
	if err != nil {
		if err == context.Canceled {
			return
		}
		log.Errorw("reading next proof failed", "err", err)
	}
}
