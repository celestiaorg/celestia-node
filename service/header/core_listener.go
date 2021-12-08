package header

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/tendermint/tendermint/types"
)

type CoreListener struct {
	sub *coreSubscription
	bc  *broadcaster
}

func NewCoreListener(ex *CoreExchange, topic *pubsub.Topic) (*CoreListener, error) {
	return &CoreListener{
		sub: newCoreSubscription(ex),
		bc:  newBroadcaster(topic),
	}, nil
}

func (cl *CoreListener) listen(ctx context.Context) {
	// start subscription to core node new block events
	err := cl.sub.start(ctx)
	if err != nil {
		log.Errorw("core-listener: starting subscription to core client", "err", err)
		return
	}

	for {
		select {
		// TODO @renaynay: should it listen for context done signal or have its own done channel?
		case <-ctx.Done():
			return
		default:
			// get next header from core
			next, err := cl.sub.NextHeader(ctx)
			if err != nil {
				log.Errorw("core-listener: getting next header", "err", err)
				return
			}
			if next == nil {
				log.Debugw("core-listener: no subsequent header, exiting listening service")
				return
			}
			// broadcast new ExtendedHeader
			err = cl.bc.Broadcast(ctx, next)
			if err != nil {
				log.Errorw("core-listener: broadcasting next header", "height", next.Height,
					"err", err)
				return
			}
		}
	}
	// kill loop when done signal sent
	// TODO make sure to call sub.Cancel() when done is sent
}

type coreSubscription struct {
	ex *CoreExchange

	sub <-chan *types.Block
}

func newCoreSubscription(ex *CoreExchange) *coreSubscription {
	return &coreSubscription{
		ex: ex,
	}
}

// start subscribes to new block events from core. It will fail if a subscription
// has already been started or if the core client is not running.
func (cs *coreSubscription) start(ctx context.Context) error {
	if cs.sub != nil {
		return fmt.Errorf("subscription already started")
	}
	if !cs.ex.fetcher.IsRunning() {
		return fmt.Errorf("core client must be running")
	}

	sub, err := cs.ex.fetcher.SubscribeNewBlockEvent(ctx)
	if err != nil {
		return err
	}
	cs.sub = sub

	return nil
}

func (cs *coreSubscription) NextHeader(ctx context.Context) (*ExtendedHeader, error) {
	select {
	case <-ctx.Done():
		return nil, nil
	case newBlock, ok := <-cs.sub:
		if !ok {
			return nil, fmt.Errorf("subscription closed")
		}
		return cs.ex.generateExtendedHeaderFromBlock(newBlock)
	}
}

func (cs *coreSubscription) Cancel() error {
	return cs.ex.fetcher.UnsubscribeNewBlockEvent(context.Background())
}
