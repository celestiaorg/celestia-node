package core

import (
	"context"
	"fmt"

	"github.com/ipfs/go-blockservice"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
)

// Listener is responsible for listening to Core for
// new block events and converting new Core blocks into
// the main data structure used in the Celestia DA network:
// `ExtendedHeader`. After digesting the Core block, extending
// it, and generating the `ExtendedHeader`, the Listener
// broadcasts the new `ExtendedHeader` to the header-sub gossipsub
// network.
type Listener struct {
	bcast     libhead.Broadcaster[*header.ExtendedHeader]
	fetcher   *core.BlockFetcher
	bServ     blockservice.BlockService
	construct header.ConstructFn
	cancel    context.CancelFunc
}

func NewListener(
	bcast libhead.Broadcaster[*header.ExtendedHeader],
	fetcher *core.BlockFetcher,
	bServ blockservice.BlockService,
	construct header.ConstructFn,
) *Listener {
	return &Listener{
		bcast:     bcast,
		fetcher:   fetcher,
		bServ:     bServ,
		construct: construct,
	}
}

// Start kicks off the Listener listener loop.
func (cl *Listener) Start(ctx context.Context) error {
	if cl.cancel != nil {
		return fmt.Errorf("listener: already started")
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

// Stop stops the Listener listener loop.
func (cl *Listener) Stop(ctx context.Context) error {
	cl.cancel()
	cl.cancel = nil
	return cl.fetcher.UnsubscribeNewBlockEvent(ctx)
}

// listen kicks off a loop, listening for new block events from Core,
// generating ExtendedHeaders and broadcasting them to the header-sub
// gossipsub network.
func (cl *Listener) listen(ctx context.Context, sub <-chan *types.Block) {
	defer log.Info("listener: listening stopped")
	for {
		select {
		case b, ok := <-sub:
			if !ok {
				return
			}

			syncing, err := cl.fetcher.IsSyncing(ctx)
			if err != nil {
				log.Errorw("listener: getting sync state", "err", err)
				return
			}

			comm, vals, err := cl.fetcher.GetBlockInfo(ctx, &b.Height)
			if err != nil {
				log.Errorw("listener: getting block info", "err", err)
				return
			}

			eh, err := cl.construct(ctx, b, comm, vals, cl.bServ)
			if err != nil {
				log.Errorw("listener: making extended header", "err", err)
				return
			}

			// broadcast new ExtendedHeader, but if core is still syncing, notify only local subscribers
			err = cl.bcast.Broadcast(ctx, eh, pubsub.WithLocalPublication(syncing))
			if err != nil {
				log.Errorw("listener: broadcasting next header", "height", eh.Height(),
					"err", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
