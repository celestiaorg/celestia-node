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
	if share.DataHash(h.DAH.Hash()).IsEmptyRoot() {
		// we still need to register it to ensure its pair gets pruned
		sp.registeredHeights <- h.Height()
		return nil
	}

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

		// we index empty DAHs because we cannot differentiate between a non-sampled height and an empty DAH otherwise.
		if !share.DataHash(dataHash).IsEmptyRoot() {
			err = sp.edsStore.Remove(ctx, dataHash)
			if err != nil {
				return err
			}
		}

		// TODO(Optimization): Queue ds deletes for batch removal?
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
		// TODO: Formalize this comment and simplify after discussion w team
		// note: this means that pruning does not happen when a new header is received, but first when that height + RecencyWindow is stored
		// the edge case here is that if a node goes offline for the length of the RecencyWindow, some blocks will not be pruned.
		// another edge case is on startup: If recent jobs are finished faster
		// than the first catchup job can retrieve the heights, those heights
		// will not get pruned. Similarly, if the network head is set too slowly
		// and catchup jobs are started outside of the recency window, those
		// heights are likely to not get pruned. My intuition here is that
		// keeping the pruning solution this simple is very valuable, and the
		// edge cases can be solved with a dumb cleanup that only needs to be
		// run manually (worst case is that they store a few extra blocks on
		// startup, or that their node has been down very long).
		case height := <-sp.registeredHeights:
			// underflow protection
			if height < sp.cfg.RecencyWindow {
				continue
			}
			// TODO: 10s is a random number
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := sp.pruneHeight(ctx, height-sp.cfg.RecencyWindow)
			cancel()
			if err != nil {
				log.Warnf("failed to prune height", "height", height, "err", err)
			}
		}
	}
}

// TODO: Figure out how to expose this to users via RPC or something. Should not
// be run in a routine because edge cases only really happen on startup, and the
// loop will run longer every time in the current implementation.
func (sp *StoragePruner) CleanupBelowHeight(ctx context.Context, height uint64) error {
	// this is slow now but when the bloom filter is implemented it will be faster than querying datastore
	for h := uint64(0); h < height; h++ {
		err := sp.pruneHeight(ctx, h)
		if err != nil {
			return fmt.Errorf("failed to prune height %d: %w", h, err)
		}
	}
	return nil
}
