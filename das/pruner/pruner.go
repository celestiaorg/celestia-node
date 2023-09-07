package pruner

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("pruner")

// TODO: Find sensible default
var defaultBufferSize = 20

type Config struct {
	RecencyWindow uint64
}

type StoragePruner struct {
	ctx    context.Context
	cancel context.CancelFunc

	cfg Config

	edsStore *eds.Store
	ds       datastore.Batching
	sa       *full.ShareAvailability

	registeredHeights chan uint64

	// TODO: hsub is not necessary because we are working under the assumption that the DASer only syncs heights inside of the recency window anyways.
	// remove after deciding completely on this assumption
	// hsub     libhead.Subscriber[*header.ExtendedHeader]
	// networkHead       uint64
}

func NewStoragePruner(
	edsStore *eds.Store,
	ds datastore.Batching,
	getter share.Getter,
	availability *full.ShareAvailability,
	config Config,
) (*StoragePruner, error) {
	return &StoragePruner{
		edsStore:          edsStore,
		ds:                ds,
		sa:                availability,
		registeredHeights: make(chan uint64, defaultBufferSize),
	}, nil
}

func (sp *StoragePruner) Start(ctx context.Context) error {
	sp.ctx, sp.cancel = context.WithCancel(context.Background())
	go sp.prune(sp.ctx)
	return nil
}

func (sp *StoragePruner) Stop(ctx context.Context) error {
	// TODO: Should we do done channels and a select to ensure services are fully stopped before returning?
	sp.cancel()
	return nil
}

func (sp *StoragePruner) SampleAndRegister(ctx context.Context, h *header.ExtendedHeader) error {
	err := sp.indexDAH(ctx, h)
	if err != nil {
		return err
	}

	err = sp.sa.SharesAvailable(ctx, h.DAH)
	if err != nil {
		return err
	}

	sp.registeredHeights <- h.Height()
	return nil
}

func (sp *StoragePruner) indexDAH(ctx context.Context, h *header.ExtendedHeader) error {
	k := datastore.NewKey(fmt.Sprintf("%d", h.Height()))
	return sp.ds.Put(ctx, k, h.DAH.Hash())
}

// note: does not set sp.lastPrunedHeight
func (sp *StoragePruner) pruneHeight(ctx context.Context, height uint64) error {
	k := datastore.NewKey(fmt.Sprintf("%d", height))
	// TODO(optimization): Use a counting bloom filter to check if the key exists in the datastore.
	// This would avoid a hit to disk, and we can remove heights from the filter as we prune them to maintain a healthy false positive rate.
	// would also maybe allow for a more robust solution for pruning ranges so that we don't need to hit the disk for every height to check
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
		// note: this means that pruning does not happen when a new header is received, but first when that height + RecencyWindow is stored
		// the edge case here is that if a node goes offline for the length of the RecencyWindow, some blocks will not be pruned.
		case height := <-sp.registeredHeights:
			// TODO: ctx timeout
			err := sp.pruneHeight(ctx, height-sp.cfg.RecencyWindow)
			if err != nil {
				log.Errorw("failed to prune height", "height", height, "err", err)
			}
		}
	}
}
