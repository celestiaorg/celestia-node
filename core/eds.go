package core

import (
	"context"
	"time"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/store"
)

// storeEDS will only store extended block if it is not empty and doesn't already exist.
func storeEDS(
	ctx context.Context,
	eh *header.ExtendedHeader,
	eds *rsmt2d.ExtendedDataSquare,
	store *store.Store,
	window time.Duration,
	archival bool,
) error {
	if !archival && !availability.IsWithinWindow(eh.Time(), window) {
		log.Debugw("skipping storage of historic block", "height", eh.Height())
		return nil
	}

	var err error
	// archival nodes should not store Q4 outside the availability window.
	if availability.IsWithinWindow(eh.Time(), window) {
		err = store.PutODSQ4(ctx, eh.DAH, eh.Height(), eds)
	} else {
		err = store.PutODS(ctx, eh.DAH, eh.Height(), eds)
	}
	if err == nil {
		log.Debugw("stored EDS for height", "height", eh.Height(), "EDS size", len(eh.DAH.RowRoots))
	}
	return err
}
