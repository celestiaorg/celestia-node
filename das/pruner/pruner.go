package pruner

import (
	"context"
	"fmt"
	"time"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("pruner")

// TODO: Find sensible default
var defaultBufferSize = 50

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
	pruneQueue        map[uint64]struct{}

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
		pruneQueue:        make(map[uint64]struct{}),
		registeredHeights: make(chan uint64, defaultBufferSize),
	}, nil
}

func (sp *StoragePruner) Start(ctx context.Context) error {
	sp.ctx, sp.cancel = context.WithCancel(context.Background())
	go sp.prune(sp.ctx)
	go sp.pruneQueueRoutine(sp.ctx)
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
		dataHash, err := sp.ds.Get(ctx, k)
		if err != nil {
			return err
		}
		// TODO: Don't remove DAH if it is empty. We index empty DAHs because we cannot differentiate between a non-sampled height and an empty DAH otherwise.
		err = sp.edsStore.Remove(ctx, dataHash)
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
		// another edge case is on startup: If recent jobs are finished faster than the first catchup job can retrieve the heights, those heights will not get pruned
		case height := <-sp.registeredHeights:
			// there are no negative heights to prune
			if height < sp.cfg.RecencyWindow {
				continue
			}
			// TODO: ctx timeout
			err := sp.pruneHeight(ctx, height-sp.cfg.RecencyWindow)
			if err != nil {
				sp.pruneQueue[height] = struct{}{}
				log.Errorw("failed to prune height, adding to prune queue", "height", height, "err", err)
			}
		}
	}
}
func (sp *StoragePruner) pruneQueueRoutine(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for height := range sp.pruneQueue {
				err := sp.pruneHeight(ctx, height)
				if err != nil {
					log.Errorw("failed to prune height from queue", "height", height, "err", err)
				} else {
					delete(sp.pruneQueue, height)
				}
			}
		}
	}
}
