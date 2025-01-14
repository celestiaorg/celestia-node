package full

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/store"
)

// ErrDisallowRevertToArchival is returned when a node has been run with pruner enabled before and
// launching it with archival mode.
var ErrDisallowRevertToArchival = errors.New(
	"node has been run with pruner enabled before, it is not safe to convert to an archival" +
		"Run with --experimental-pruning enabled or consider re-initializing the store")

var (
	log         = logging.Logger("share/full")
	storePrefix = datastore.NewKey("full_avail")
)

// ShareAvailability implements share.Availability using the full data square
// recovery technique. It is considered "full" because it is required
// to download enough shares to fully reconstruct the data square.
type ShareAvailability struct {
	store  *store.Store
	getter shwap.Getter

	ds datastore.Datastore

	storageWindow time.Duration
	archival      bool
}

// NewShareAvailability creates a new full ShareAvailability.
func NewShareAvailability(
	store *store.Store,
	getter shwap.Getter,
	ds datastore.Datastore,
	opts ...Option,
) *ShareAvailability {
	p := defaultParams()
	for _, opt := range opts {
		opt(p)
	}

	return &ShareAvailability{
		store:         store,
		getter:        getter,
		ds:            namespace.Wrap(ds, storePrefix),
		storageWindow: availability.StorageWindow,
		archival:      p.archival,
	}
}

// SharesAvailable reconstructs the data committed to the given Root by requesting
// enough Shares from the network.
func (fa *ShareAvailability) SharesAvailable(ctx context.Context, header *header.ExtendedHeader) error {
	dah := header.DAH

	if !fa.archival {
		// do not sync blocks outside of sampling window if not archival
		if !availability.IsWithinWindow(header.Time(), fa.storageWindow) {
			log.Debugw("skipping availability check for block outside sampling"+
				" window", "height", header.Height(), "data hash", dah.String())
			return availability.ErrOutsideSamplingWindow
		}
	}

	// if the data square is empty, we can safely link the header height in the store to an empty EDS.
	if share.DataHash(dah.Hash()).IsEmptyEDS() {
		err := fa.store.PutODSQ4(ctx, dah, header.Height(), share.EmptyEDS())
		if err != nil {
			return fmt.Errorf("put empty EDS: %w", err)
		}
		return nil
	}

	// a hack to avoid loading the whole EDS in mem if we store it already.
	if ok, _ := fa.store.HasByHeight(ctx, header.Height()); ok {
		return nil
	}

	eds, err := fa.getter.GetEDS(ctx, header)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		log.Errorw("availability validation failed", "root", dah.String(), "err", err.Error())
		var byzantineErr *byzantine.ErrByzantine
		if errors.Is(err, shwap.ErrNotFound) || errors.Is(err, context.DeadlineExceeded) && !errors.As(err, &byzantineErr) {
			return share.ErrNotAvailable
		}
		return err
	}

	// archival nodes should not store Q4 outside the availability window.
	if availability.IsWithinWindow(header.Time(), fa.storageWindow) {
		err = fa.store.PutODSQ4(ctx, dah, header.Height(), eds)
	} else {
		err = fa.store.PutODS(ctx, dah, header.Height(), eds)
	}

	if err != nil {
		return fmt.Errorf("full availability: failed to store eds: %w", err)
	}
	return nil
}

func (fa *ShareAvailability) Prune(ctx context.Context, eh *header.ExtendedHeader) error {
	if fa.archival {
		log.Debugf("trimming Q4 from block %s at height %d", eh.DAH.String(), eh.Height())
		return fa.store.RemoveQ4(ctx, eh.Height(), eh.DAH.Hash())
	}

	log.Debugf("removing block %s at height %d", eh.DAH.String(), eh.Height())
	return fa.store.RemoveODSQ4(ctx, eh.Height(), eh.DAH.Hash())
}

var (
	previousModeKey = datastore.NewKey("previous_run")
	pruned          = []byte("pruned")
	archival        = []byte("archival")
)

// ConvertFromArchivalToPruned ensures that a node has not been run with pruning enabled before
// cannot revert to archival mode. It returns true only if the node is converting to
// pruned mode for the first time.
func (fa *ShareAvailability) ConvertFromArchivalToPruned(ctx context.Context) (bool, error) {
	prevMode, err := fa.ds.Get(ctx, previousModeKey)
	if err != nil {
		return false, err
	}

	if bytes.Equal(prevMode, pruned) && fa.archival {
		return false, ErrDisallowRevertToArchival
	}

	if bytes.Equal(prevMode, archival) && !fa.archival {
		// allow conversion from archival to pruned
		err = fa.ds.Put(ctx, previousModeKey, pruned)
		if err != nil {
			return false, fmt.Errorf("share/availability/full: failed to updated pruning mode in "+
				"datastore: %w", err)
		}
		return true, nil
	}

	// no changes in pruning mode
	return false, nil
}

// DetectFirstRun is a temporary function that serves to assist migration to the refactored pruner
// implementation (v0.21.0). It checks if the node has been run with pruning enabled before by checking
// if the pruner service ran before, and disallows running as an archival node in the case it has.
//
// TODO @renaynay: remove this function after a few releases.
func DetectFirstRun(ctx context.Context, fa *ShareAvailability, lastPrunedHeight uint64) error {
	exists, err := fa.ds.Has(ctx, previousModeKey)
	if err != nil {
		return fmt.Errorf("share/availability/full: failed to check previous pruned run in "+
			"datastore: %w", err)
	}
	if exists {
		return nil
	}

	if fa.archival {
		if lastPrunedHeight > 1 {
			return ErrDisallowRevertToArchival
		}

		return fa.ds.Put(ctx, previousModeKey, archival)
	}

	return fa.ds.Put(ctx, previousModeKey, pruned)
}
