package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/tendermint/tendermint/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

var tracer = otel.Tracer("core/listener")

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
	store     *eds.Store

	headerBroadcaster libhead.Broadcaster[*header.ExtendedHeader]
	hashBroadcaster   shrexsub.BroadcastFn

	listenerTimeout time.Duration

	cancel context.CancelFunc
}

func NewListener(
	bcast libhead.Broadcaster[*header.ExtendedHeader],
	fetcher *BlockFetcher,
	hashBroadcaster shrexsub.BroadcastFn,
	construct header.ConstructFn,
	store *eds.Store,
	blocktime time.Duration,
) *Listener {
	return &Listener{
		fetcher:           fetcher,
		headerBroadcaster: bcast,
		hashBroadcaster:   hashBroadcaster,
		construct:         construct,
		store:             store,
		listenerTimeout:   2 * blocktime,
	}
}

// Start kicks off the Listener listener loop.
func (cl *Listener) Start(context.Context) error {
	if cl.cancel != nil {
		return fmt.Errorf("listener: already started")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cl.cancel = cancel

	sub, err := cl.fetcher.SubscribeNewBlockEvent(ctx)
	if err != nil {
		return err
	}
	go cl.runSubscriber(ctx, sub)
	return nil
}

// Stop stops the listener loop.
func (cl *Listener) Stop(context.Context) error {
	cl.cancel()
	cl.cancel = nil
	return nil
}

// runSubscriber runs a subscriber to receive event data of new signed blocks. It will attempt to
// resubscribe in case error happens during listening of subscription
func (cl *Listener) runSubscriber(ctx context.Context, sub <-chan types.EventDataSignedBlock) {
	for {
		err := cl.listen(ctx, sub)
		if ctx.Err() != nil {
			// listener stopped because external context was canceled
			return
		}
		log.Warnw("listener: subscriber error, resubscribing...", "err", err)

		err = cl.fetcher.UnsubscribeNewBlockEvent(ctx)
		if err != nil {
			log.Errorw("listener: unsubscribe error", "err", err)
			return
		}

		sub, err = cl.fetcher.SubscribeNewBlockEvent(ctx)
		if err != nil {
			log.Errorw("listener: resubscribe error", "err", err)
			return
		}
	}
}

// listen kicks off a loop, listening for new block events from Core,
// generating ExtendedHeaders and broadcasting them to the header-sub
// gossipsub network.
func (cl *Listener) listen(ctx context.Context, sub <-chan types.EventDataSignedBlock) error {
	defer log.Info("listener: listening stopped")
	timeout := time.NewTimer(cl.listenerTimeout)
	defer timeout.Stop()
	for {
		select {
		case b, ok := <-sub:
			if !ok {
				return errors.New("underlying subscription was closed")
			}

			log.Debugw("listener: new block from core", "height", b.Header.Height)
			err := cl.handleNewSignedBlock(ctx, b)
			if err != nil {
				log.Errorw("listener: handling new block msg",
					"height", b.Header.Height,
					"hash", b.Header.Hash().String(),
					"err", err)
			}

			if !timeout.Stop() {
				<-timeout.C
			}
			timeout.Reset(cl.listenerTimeout)
		case <-timeout.C:
			return errors.New("underlying subscription is stuck")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (cl *Listener) handleNewSignedBlock(ctx context.Context, b types.EventDataSignedBlock) error {
	ctx, span := tracer.Start(ctx, "handle-new-signed-block")
	defer span.End()
	span.SetAttributes(
		attribute.Int64("height", b.Header.Height),
	)
	// extend block data
	adder := ipld.NewProofsAdder(int(b.Data.SquareSize))
	defer adder.Purge()

	eds, err := extendBlock(b.Data, b.Header.Version.App, nmt.NodeVisitor(adder.VisitFn()))
	if err != nil {
		return fmt.Errorf("extending block data: %w", err)
	}
	// generate extended header
	eh, err := cl.construct(&b.Header, &b.Commit, &b.ValidatorSet, eds)
	if err != nil {
		panic(fmt.Errorf("making extended header: %w", err))
	}

	// attempt to store block data if not empty
	ctx = ipld.CtxWithProofsAdder(ctx, adder)
	err = storeEDS(ctx, b.Header.DataHash.Bytes(), eds, cl.store)
	if err != nil {
		return fmt.Errorf("storing EDS: %w", err)
	}

	syncing, err := cl.fetcher.IsSyncing(ctx)
	if err != nil {
		return fmt.Errorf("getting sync state: %w", err)
	}

	// notify network of new EDS hash only if core is already synced
	if !syncing {
		err = cl.hashBroadcaster(ctx, shrexsub.Notification{
			DataHash: eh.DataHash.Bytes(),
			Height:   eh.Height(),
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Errorw("listener: broadcasting data hash",
				"height", b.Header.Height,
				"hash", b.Header.Hash(), "err", err) //TODO: hash or datahash?
		}
	}

	// broadcast new ExtendedHeader, but if core is still syncing, notify only local subscribers
	err = cl.headerBroadcaster.Broadcast(ctx, eh, pubsub.WithLocalPublication(syncing))
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Errorw("listener: broadcasting next header",
			"height", b.Header.Height,
			"err", err)
	}
	return nil
}
