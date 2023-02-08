package share

import (
	"bytes"
	"context"
	"fmt"
	"math"

	"github.com/ipfs/go-blockservice"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/rsmt2d"
)

// EnsureEmptySquareExists checks if the given DAG contains an empty block data square.
// If it does not, it stores an empty block. This optimization exists to prevent
// redundant storing of empty block data so that it is only stored once and returned
// upon request for a block with an empty data square. Ref: header/constructors.go#L56
func EnsureEmptySquareExists(ctx context.Context, bServ blockservice.BlockService) (*rsmt2d.ExtendedDataSquare, error) {
	return AddShares(ctx, emptyDataSquare(), bServ)
}

// EmptyExtendedDataSquare returns the EDS of the empty block data square.
func EmptyExtendedDataSquare() *rsmt2d.ExtendedDataSquare {
	shares := emptyDataSquare()
	squareSize := uint64(math.Sqrt(float64(appconsts.DefaultMinSquareSize)))
	eds, err := da.ExtendShares(squareSize, shares)
	if err != nil {
		panic(fmt.Errorf("failed to create empty EDS: %w", err))
	}
	return eds
}

// tail is filler for all tail padded shares
// it is allocated once and used everywhere
var tailPaddingShare = append(
	append(make([]byte, 0, appconsts.ShareSize), appconsts.TailPaddingNamespaceID...),
	bytes.Repeat([]byte{0}, appconsts.ShareSize-appconsts.NamespaceSize)...,
)

// emptyDataSquare returns the minimum size data square filled with tail padding.
func emptyDataSquare() [][]byte {
	shares := make([][]byte, appconsts.MinShareCount)
	for i := 0; i < appconsts.MinShareCount; i++ {
		shares[i] = tailPaddingShare
	}

	return shares
}
