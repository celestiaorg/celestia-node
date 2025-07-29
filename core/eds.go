package core

import (
	"context"
	"fmt"
	"time"

	coretypes "github.com/cometbft/cometbft/types"

	"github.com/celestiaorg/celestia-app/v5/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v5/pkg/wrapper"
	libsquare "github.com/celestiaorg/go-square/v2"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/store"
)

// isEmptyBlockRef returns true if the application considers the given block data
// empty at a given version.
func isEmptyBlockRef(data *coretypes.Data) bool {
	return len(data.Txs) == 0
}

// extendBlock extends the given block data, returning the resulting
// ExtendedDataSquare (EDS). If there are no transactions in the block,
// nil is returned in place of the eds.
func extendBlock(data *coretypes.Data, options ...nmt.Option) (*rsmt2d.ExtendedDataSquare, error) {
	if isEmptyBlockRef(data) {
		return share.EmptyEDS(), nil
	}

	// Construct the data square from the block's transactions
	square, err := libsquare.Construct(
		data.Txs.ToSliceOfBytes(),
		appconsts.SquareSizeUpperBound,
		appconsts.SubtreeRootThreshold,
	)
	if err != nil {
		return nil, err
	}
	return extendShares(libshare.ToBytes(square), options...)
}

func extendShares(s [][]byte, options ...nmt.Option) (*rsmt2d.ExtendedDataSquare, error) {
	// Check that the length of the square is a power of 2.
	if !libsquare.IsPowerOfTwo(len(s)) {
		return nil, fmt.Errorf("number of shares is not a power of 2: got %d", len(s))
	}
	// here we construct a tree
	// Note: uses the nmt wrapper to construct the tree.
	squareSize := libsquare.Size(len(s))
	return rsmt2d.ComputeExtendedDataSquare(s,
		appconsts.DefaultCodec(),
		wrapper.NewConstructor(uint64(squareSize),
			options...))
}

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
		log.Debugw("stored EDS for height", "height", eh.Height())
	}
	return err
}
