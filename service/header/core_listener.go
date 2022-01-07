package header

import (
	"context"
	"fmt"

	format "github.com/ipfs/go-ipld-format"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-node/core"
)

// CoreListener is responsible for listening to Core for
// new block events and converting new Core blocks into
// the main data structure used in the Celestia DA network:
// `ExtendedHeader`. After digesting the Core block, extending
// it, and generating the `ExtendedHeader`, the CoreListener
// broadcasts the new `ExtendedHeader` to the header-sub gossipsub
// network.
type CoreListener struct {
	p2pSub  *P2PSubscriber
	fetcher *core.BlockFetcher
	dag     format.DAGService

	blockSub <-chan *types.Block

	ctx    context.Context
	cancel context.CancelFunc
}

func NewCoreListener(p2pSub *P2PSubscriber, fetcher *core.BlockFetcher, dag format.DAGService) *CoreListener {
	return &CoreListener{
		p2pSub:  p2pSub,
		fetcher: fetcher,
		dag:     dag,
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
	cl.ctx, cl.cancel = nil, nil
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
	cl.blockSub, err = cl.fetcher.SubscribeNewBlockEvent(ctx)

	return err
}

// cancelBlockSubscription stops the CoreListener's subscription to new block events
// from Core.
func (cl *CoreListener) cancelBlockSubscription(ctx context.Context) error {
	return cl.fetcher.UnsubscribeNewBlockEvent(ctx)
}

// nextHeader returns the next latest header from Core.
func (cl *CoreListener) nextHeader(ctx context.Context) (*ExtendedHeader, error) {
	select {
	case newBlock, ok := <-cl.blockSub:
		if !ok {
			return nil, fmt.Errorf("subscription closed")
		}

		comm, vals, err := cl.fetcher.GetBlockInfo(ctx, &newBlock.Height)
		if err != nil {
			return nil, err
		}

		return MakeExtendedHeader(ctx, newBlock, comm, vals, cl.dag)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
