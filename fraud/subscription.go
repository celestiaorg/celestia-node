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

// OnProof subscribes on a single Fraud Proof.
// In case a Fraud Proof is received, then the given handle function will be invoked.
func OnProof(ctx context.Context, subscriber Subscriber, p ProofType, handle func(proof Proof)) {
	subscription, err := subscriber.Subscribe(p)
	if err != nil {
		log.Error(err)
		return
	}
	defer subscription.Cancel()

	// At this point we receive already verified fraud proof,
	// so there are no needs to call Validate.
	proof, err := subscription.Proof(ctx)
	if err != nil {
		if err != context.Canceled {
			log.Errorw("reading next proof failed", "err", err)
		}
		return
	}

	handle(proof)
}
