package header

import (
	"context"
	"fmt"

	format "github.com/ipfs/go-ipld-format"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
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
	bcast   Broadcaster
	fetcher *core.BlockFetcher
	dag     format.DAGService

	cancel context.CancelFunc
}

func NewCoreListener(bcast Broadcaster, fetcher *core.BlockFetcher, dag format.DAGService) *CoreListener {
	return &CoreListener{
		bcast:   bcast,
		fetcher: fetcher,
		dag:     dag,
	}
}

// Start kicks off the CoreListener listener loop.
func (cl *CoreListener) Start(ctx context.Context) error {
	if cl.cancel != nil {
		return fmt.Errorf("core-listener: already started")
	}

	sub, err := cl.fetcher.SubscribeNewBlockEvent(ctx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	go cl.listen(ctx, sub)
	cl.cancel = cancel
	return nil
}

// Stop stops the CoreListener listener loop.
func (cl *CoreListener) Stop(ctx context.Context) error {
	cl.cancel()
	cl.cancel = nil
	return cl.fetcher.UnsubscribeNewBlockEvent(ctx)
}

// listen kicks off a loop, listening for new block events from Core,
// generating ExtendedHeaders and broadcasting them to the header-sub
// gossipsub network.
func (cl *CoreListener) listen(ctx context.Context, sub <-chan *types.Block) {
	defer log.Info("core-listener: listening stopped")
	for {
		select {
		case b, ok := <-sub:
			if !ok {
				return
			}

			syncing, err := cl.fetcher.IsSyncing(ctx)
			if err != nil {
				log.Errorw("core-listener: getting sync state", "err", err)
				return
			}

			comm, vals, err := cl.fetcher.GetBlockInfo(ctx, &b.Height)
			if err != nil {
				log.Errorw("core-listener: getting block info", "err", err)
				return
			}

			eh, err := MakeExtendedHeader(ctx, b, comm, vals, cl.dag)
			if err != nil {
				log.Errorw("core-listener: making extended header", "err", err)
				return
			}

			// broadcast new ExtendedHeader, but if core is still syncing, notify only local subscribers
			err = cl.bcast.Broadcast(ctx, eh, pubsub.WithLocalPublication(syncing))
			if err != nil {
				log.Errorw("core-listener: broadcasting next header", "height", eh.Height,
					"err", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
