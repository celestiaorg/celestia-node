package core

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

// Listener is responsible for listening to Core for
// new block events and converting new Core blocks into
// the main data structure used in the Celestia DA network:
// `ExtendedHeader`. After digesting the Core block, extending
// it, and generating the `ExtendedHeader`, the Listener
// broadcasts the new `ExtendedHeader` to the header-sub gossipsub
// network.
type Listener struct {
	fetcher *BlockFetcher

	construct header.ConstructFn

	store *eds.Store

	headerBroadcaster libhead.Broadcaster[*header.ExtendedHeader]
	hashBroadcaster   shrexsub.BroadcastFn

	cancel context.CancelFunc
}

func NewListener(
	bcast libhead.Broadcaster[*header.ExtendedHeader],
	fetcher *BlockFetcher,
	extHeaderBroadcaster shrexsub.BroadcastFn,
	construct header.ConstructFn,
	store *eds.Store,
) *Listener {
	return &Listener{
		headerBroadcaster: bcast,
		fetcher:           fetcher,
		hashBroadcaster:   extHeaderBroadcaster,
		construct:         construct,
		store:             store,
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

// Stop stops the listener loop.
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

			// extend block data
			eds, err := extendBlock(b.Data)
			if err != nil {
				log.Errorw("listener: extending block data", "err", err)
				return
			}
			// generate extended header
			eh, err := cl.construct(ctx, b, comm, vals, eds)
			if err != nil {
				log.Errorw("listener: making extended header", "err", err)
				return
			}
			// store block data if not empty
			if eds != nil {
				err = cl.store.Put(ctx, eh.DAH.Hash(), eds)
				if err != nil {
					log.Errorw("listener: storing extended header", "err", err)
					return
				}
			}

			// broadcast new ExtendedHeader, but if core is still syncing, notify only local subscribers
			err = cl.headerBroadcaster.Broadcast(ctx, eh, pubsub.WithLocalPublication(syncing))
			if err != nil {
				log.Errorw("listener: broadcasting next header", "height", eh.Height(),
					"err", err)
			}

			// notify network of new EDS hash only if core is already synced
			if !syncing {
				err = cl.hashBroadcaster(ctx, eh.DataHash.Bytes())
				if err != nil {
					log.Errorw("listener: broadcasting data hash", "height", eh.Height(),
						"hash", eh.Hash(), "err", err)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
