package header

import (
	"context"
	"errors"
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
	if cl.blockSub != nil {
		return fmt.Errorf("core-listener: already started")
	}

	sub, err := cl.fetcher.SubscribeNewBlockEvent(ctx)
	if err != nil {
		return err
	}

	cl.ctx, cl.cancel = context.WithCancel(context.Background())
	cl.blockSub = sub
	go cl.listen()
	return nil
}

// Stop stops the CoreListener listener loop.
func (cl *CoreListener) Stop(ctx context.Context) error {
	cl.cancel()
	cl.blockSub = nil
	return cl.fetcher.UnsubscribeNewBlockEvent(ctx)
}

// listen kicks off a loop, listening for new block events from Core,
// generating ExtendedHeaders and broadcasting them to the header-sub
// gossipsub network.
func (cl *CoreListener) listen() {
	defer log.Info("core-listener: listening stopped")
	for {
		select {
		case b, ok := <-cl.blockSub:
			if !ok {
				return
			}

			comm, vals, err := cl.fetcher.GetBlockInfo(cl.ctx, &b.Height)
			if err != nil {
				log.Errorw("core-listener: getting block info", "err", err)
				return
			}

			eh, err := MakeExtendedHeader(cl.ctx, b, comm, vals, cl.dag)
			if err != nil {
				log.Errorw("core-listener: making extended header", "err", err)
				return
			}

			// broadcast new ExtendedHeader
			err = cl.p2pSub.Broadcast(cl.ctx, eh)
			if err != nil {
				var pserr pubsub.ValidationError
				if errors.As(err, &pserr) && pserr.Reason == pubsub.RejectValidationIgnored {
					log.Warnw("core-listener: broadcasting next header", "height", eh.Height,
						"err", err)
				} else {
					log.Errorw("core-listener: broadcasting next header", "height", eh.Height,
						"err", err)
				}

				return
			}
		case <-cl.ctx.Done():
			return
		}
	}
}
