package shrexsub

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/celestiaorg/celestia-node/share"
)

// Subscription is a wrapper over pubsub.Subscription that handles
// receiving an EDS DataHash from other peers.
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

// Next blocks the caller until any new EDS DataHash notification arrives.
// Returns only notifications which successfully pass validation.
func (subs *Subscription) Next(ctx context.Context) (share.DataHash, error) {
	msg, err := subs.subscription.Next(ctx)
	if err != nil {
		log.Errorw("listening for the next eds hash", "err", err)
		return nil, err
	}

	log.Debugw("received message", "topic", msg.Message.GetTopic(), "sender", msg.ReceivedFrom)
	return msg.Data, nil
}

// Cancel stops the subscription.
func (subs *Subscription) Cancel() {
	subs.subscription.Cancel()
}
