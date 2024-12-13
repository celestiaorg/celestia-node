package full

import (
	"context"
	"errors"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/store"
)

var log = logging.Logger("share/full")

// ShareAvailability implements share.Availability using the full data square
// recovery technique. It is considered "full" because it is required
// to download enough shares to fully reconstruct the data square.
type ShareAvailability struct {
	store  *store.Store
	getter shwap.Getter

	storageWindow time.Duration
	archival      bool
}

// NewShareAvailability creates a new full ShareAvailability.
func NewShareAvailability(
	store *store.Store,
	getter shwap.Getter,
	opts ...Option,
) *ShareAvailability {
	p := defaultParams()
	for _, opt := range opts {
		opt(p)
	}

	return &ShareAvailability{
		store:         store,
		getter:        getter,
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
