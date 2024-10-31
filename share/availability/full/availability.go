package full

import (
	"context"
	"errors"
	"fmt"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/pruner/full"
	"github.com/celestiaorg/celestia-node/share"
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
}

// NewShareAvailability creates a new full ShareAvailability.
func NewShareAvailability(
	store *store.Store,
	getter shwap.Getter,
) *ShareAvailability {
	return &ShareAvailability{
		store:  store,
		getter: getter,
	}
}

// SharesAvailable reconstructs the data committed to the given Root by requesting
// enough Shares from the network.
func (fa *ShareAvailability) SharesAvailable(ctx context.Context, header *header.ExtendedHeader) error {
	dah := header.DAH
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
	if pruner.IsWithinAvailabilityWindow(header.Time(), full.Window) {
		err = fa.store.PutODSQ4(ctx, dah, header.Height(), eds)
	} else {
		err = fa.store.PutODS(ctx, dah, header.Height(), eds)
	}

	if err != nil {
		return fmt.Errorf("full availability: failed to store eds: %w", err)
	}
	return nil
}
