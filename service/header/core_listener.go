package header

import (
	"context"
	"errors"
	"fmt"

	format "github.com/ipfs/go-ipld-format"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

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
func (cl *CoreListener) Start(context.Context) error {
	if cl.cancel != nil {
		return fmt.Errorf("core-listener: already started")
	}

	ctx, cancel := context.WithCancel(context.Background())

	go cl.listen(ctx)
	cl.cancel = cancel
	return nil
}

// Stop stops the CoreListener listener loop.
func (cl *CoreListener) Stop(ctx context.Context) error {
	cl.cancel()
	cl.cancel = nil

	err := cl.fetcher.UnsubscribeNewBlockEvent(ctx)
	if err != nil {
		log.Errorw("core-listener: failed to unsubscribe from new block events", "err", err)
	}
	return nil
}

// listen kicks off a loop, listening for new block events from Core,
// generating ExtendedHeaders and broadcasting them to the header-sub
// gossipsub network.
func (cl *CoreListener) listen(ctx context.Context) {
	defer log.Info("core-listener: listening stopped")

	// listener loop will only begin once the Core connection has finished
	// syncing in order to prevent spamming of old block headers to `headersub`
	if err := cl.fetcher.WaitFinishSync(ctx); err != nil {
		// TODO @renaynay: should this be a fatal error? if the core listener cannot start, then the bridge node is
		// 	useless as it won't be broadcasting new headers to the network.
		log.Errorw("core-listener: listener loop failed to start", "err", err)
	}

	// all caught up, start broadcasting new headers
	sub, err := cl.fetcher.SubscribeNewBlockEvent(ctx)
	if err != nil {
		log.Errorw("core-listener: failed to subscribe to new block events", "err", err)
		return
	}

	for {
		select {
		case b, ok := <-sub:
			if !ok {
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

			// broadcast new ExtendedHeader
			err = cl.bcast.Broadcast(ctx, eh)
			if err != nil {
				var pserr pubsub.ValidationError
				// TODO(@Wondertan): Log ValidationIgnore cases as well, once headr duplication issue is fixed
				if errors.As(err, &pserr) && pserr.Reason != pubsub.RejectValidationIgnored {
					log.Errorw("core-listener: broadcasting next header", "height", eh.Height,
						"err", err)
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
