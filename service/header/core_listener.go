package header

import (
	"context"
	"fmt"

	"github.com/tendermint/tendermint/types"
)

// CoreListener TODO @renaynay: document
type CoreListener struct {
	ex     *CoreExchange
	p2pSub *P2PSubscriber

	blockSub <-chan *types.Block

	ctx    context.Context
	cancel context.CancelFunc
}

func NewCoreListener(ex *CoreExchange, p2pSub *P2PSubscriber) *CoreListener {
	return &CoreListener{
		ex:     ex,
		p2pSub: p2pSub,
	}
}

// Start kicks off the CoreListener listener loop.
func (cl *CoreListener) Start(ctx context.Context) error {
	cl.ctx, cl.cancel = context.WithCancel(context.Background())

	go cl.listen(ctx)
	return nil
}

// Stop stops the CoreListener listener loop.
func (cl *CoreListener) Stop(context.Context) error {
	cl.cancel()
	return nil
}

// listen kicks off a loop, listening for new block events from Core,
// generating ExtendedHeaders and broadcasting them to the header-sub
// gossipsub network.
func (cl *CoreListener) listen(ctx context.Context) {
	// start subscription to core node new block events
	err := cl.startBlockSubscription(ctx)
	if err != nil {
		log.Errorw("core-listener: starting subscription to core client", "err", err)
		return
	}

	for {
		select {
		case <-cl.ctx.Done():
			if err := cl.cancelBlockSubscription(context.Background()); err != nil {
				log.Errorw("core-listener: canceling subscription to core", "err", err)
			}
			return
		default:
			// get next header from core
			next, err := cl.nextHeader(cl.ctx)
			if err != nil {
				log.Errorw("core-listener: getting next header", "err", err)
				return
			}
			if next == nil {
				log.Debugw("core-listener: no subsequent header, exiting listening service")
				return
			}
			// broadcast new ExtendedHeader
			err = cl.p2pSub.Broadcast(cl.ctx, next)
			if err != nil {
				log.Errorw("core-listener: broadcasting next header", "height", next.Height,
					"err", err)
				return
			}
		}
	}
}

// startBlockSubscription starts the CoreListener's subscription to new block events
// from Core.
func (cl *CoreListener) startBlockSubscription(ctx context.Context) error {
	if cl.blockSub != nil {
		return fmt.Errorf("subscription already started")
	}

	var err error
	cl.blockSub, err = cl.ex.fetcher.SubscribeNewBlockEvent(ctx)

	return err
}

// cancelBlockSubscription stops the CoreListener's subscription to new block events
// from Core.
func (cl *CoreListener) cancelBlockSubscription(ctx context.Context) error {
	return cl.ex.fetcher.UnsubscribeNewBlockEvent(ctx)
}

// nextHeader returns the next latest header from Core.
func (cl *CoreListener) nextHeader(ctx context.Context) (*ExtendedHeader, error) {
	select {
	case <-ctx.Done():
		return nil, nil
	case newBlock, ok := <-cl.blockSub:
		if !ok {
			return nil, fmt.Errorf("subscription closed")
		}
		return cl.ex.generateExtendedHeaderFromBlock(newBlock)
	}
}
