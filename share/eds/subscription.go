package eds

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// Subscription is a wrapper over pubsub.Subscription that handles
// receiving an EDS from other peers.
type Subscription struct {
	subscription *pubsub.Subscription
}

func newSubscription(t *pubsub.Topic) (*Subscription, error) {
	subs, err := t.Subscribe()
	if err != nil {
		return nil, err
	}

	return &Subscription{subscription: subs}, nil
}

// Next blocks the callee until any new EDS notification(data hash) arrives.
// Returns only notifications which successfully went through validation pipeline.
func (subs *Subscription) Next(ctx context.Context) ([]byte, error) {
	msg, err := subs.subscription.Next(ctx)
	if err != nil {
		log.Errorw("listening to the next eds", "err", err)
		return nil, err
	}

	log.Debugw("received message", "topic", msg.Message.GetTopic(), "sender", msg.ReceivedFrom)
	return msg.Data, nil
}

// Cancel stops the subscription.
func (subs *Subscription) Cancel() {
	subs.subscription.Cancel()
}
