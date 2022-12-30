package das

import (
	"context"
	headerpkg "github.com/celestiaorg/celestia-node/libs/header"

	"github.com/celestiaorg/celestia-node/header"
)

// subscriber subscribes to notifications about new headers in the network to keep
// sampling process up-to-date with current network state.
type subscriber struct {
	done
}

func newSubscriber() subscriber {
	return subscriber{newDone("subscriber")}
}

func (s *subscriber) run(ctx context.Context, sub headerpkg.Subscription[*header.ExtendedHeader], emit listenFn) {
	defer s.indicateDone()
	defer sub.Cancel()

	for {
		h, err := sub.NextHeader(ctx)
		if err != nil {
			if err == context.Canceled {
				return
			}

			log.Errorw("failed to get next header", "err", err)
			continue
		}
		log.Infow("new header received via subscription", "height", h.Height())

		emit(ctx, uint64(h.Height()))
	}
}
