package pruner

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/getters"
	"github.com/celestiaorg/celestia-node/share/p2p/discovery"
	libhead "github.com/celestiaorg/go-header"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("pruner")

// TODO: Find sensible default
var defaultBufferSize = 20

type Config struct {
	RecencyWindow uint64
	BatchSize     uint64
}

type StoragePruner struct {
	ctx    context.Context
	cancel context.CancelFunc

	cfg *Config

	edsStore *eds.Store
	hsub     libhead.Subscriber[*header.ExtendedHeader]
	ds       datastore.Batching

	// TO DISCUSS: Maybe we should just modify full ShareAvailability to have methods for both GetAndPut and Get, and then use full Avail directly
	getter *full.ShareAvailability
	putter *full.ShareAvailability

	networkHead uint64
	registeredHeights chan uint64
}

func NewStoragePruner(
	edsStore *eds.Store,
	hsub libhead.Subscriber[*header.ExtendedHeader],
	ds datastore.Batching,
	disc *discovery.Discovery,
	getter share.Getter,
	originalAvail *full.ShareAvailability,
) (*StoragePruner, error) {
	teeGetter := getters.NewTeeGetter(getter, edsStore)
	return &StoragePruner{
		edsStore:          edsStore,
		hsub:              hsub,
		ds:                ds,
		getter:            originalAvail,
		putter:            full.NewShareAvailability(edsStore, teeGetter, disc),
		registeredHeights: make(chan uint64, defaultBufferSize),
	}, nil
}

func (sp *StoragePruner) Start(ctx context.Context) error {
	sp.ctx, sp.cancel = context.WithCancel(context.Background())

	sub, err := sp.hsub.Subscribe()
	if err != nil {
		return err
	}

	go sp.listenForHeaders(sp.ctx, sub)
	go sp.prune(sp.ctx)
	return nil
}

func (sp *StoragePruner) Stop(ctx context.Context) error {
	// TODO: Should we do done channels and a select to ensure services are fully stopped before returning?
	sp.cancel()
	return nil
}

func (sp *StoragePruner) SampleAndRegister(ctx context.Context, h *header.ExtendedHeader) error {
	if sp.isRecent(h.Height()) {
		err := sp.indexDAH(ctx, h)
		if err != nil {
			return err
		}
		// data only gets tee'd to storage if it is inside recency window
		err = sp.putter.SharesAvailable(ctx, h.DAH)
		if err != nil {
			return err
		}

		sp.registeredHeights <- h.Height()
		return nil
	}

	err := sp.getter.SharesAvailable(ctx, h.DAH)
	if err != nil {
		return err
	}
	// even heights that are not stored must be registered on the channel, so that the pruner knows to prune h.Height() - RecencyWindow
	sp.registeredHeights <- h.Height()
}

func (sp *StoragePruner) indexDAH(ctx context.Context, h *header.ExtendedHeader) error {
	k := datastore.NewKey(fmt.Sprintf("%d", h.Height()))
	return sp.ds.Put(ctx, k, h.DAH.Hash())
}

func (sp *StoragePruner) isRecent(height uint64) bool {
	return height > sp.networkHead-sp.cfg.RecencyWindow
}

// note: does not set sp.lastPrunedHeight
func (sp *StoragePruner) pruneHeight(ctx context.Context, height uint64) error {
	k := datastore.NewKey(fmt.Sprintf("%d", height))
	// TODO(optimization): Use a counting bloom filter to check if the key exists in the datastore.
	// This would avoid a hit to disk, and we can remove heights from the filter as we prune them to maintain a healthy false positive rate.
	exists, err := sp.ds.Has(ctx, k)
	if err != nil {
		return err
	}

	if exists {
		dah, err := sp.ds.Get(ctx, k)
		if err != nil {
			return err
		}
		err = sp.edsStore.Remove(ctx, dah)
		if err != nil {
			return err
		}

		// TODO(Optimization): Queue ds deletes for batch removal
		err = sp.ds.Delete(ctx, k)
		if err != nil {
			return err
		}
	}
	return nil
}

func (sp *StoragePruner) prune(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		// TODO: Investigate edge cases where a height may not be removed.
		// note: this means that pruning does not happen when a new header is received, but first when that height is stored
		case height := <-sp.registeredHeights:
			// TODO: batches
			// TODO: ctx timeout
			err := sp.pruneHeight(ctx, height-sp.cfg.RecencyWindow)
			log.Errorw("failed to prune height", "height", height, "err", err)
		}
	}
}

func (sp *StoragePruner) listenForHeaders(ctx context.Context, sub libhead.Subscription[*header.ExtendedHeader]) {
	for {
		h, err := sub.NextHeader(ctx)
		if err != nil {
			if err == context.Canceled {
				return
			}

			log.Errorw("failed to get next header", "err", err)
			continue
		}
		newHeight := h.Height()
		if newHeight <= sp.networkHead {
			log.Warnf("received head height: %v, which is lower or the same as previously known: %v", newHeight, sp.networkHead)
			continue
		}

		sp.networkHead = newHeight
	}
}
