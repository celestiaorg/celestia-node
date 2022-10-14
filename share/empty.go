package share

import (
	"bytes"
	"context"

	"github.com/ipfs/go-blockservice"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
)

// EnsureEmptySquareExists checks if the given DAG contains an empty block data square.
// If it does not, it stores an empty block. This optimization exists to prevent
// redundant storing of empty block data so that it is only stored once and returned
// upon request for a block with an empty data square. Ref: header/header.go#L56
func EnsureEmptySquareExists(ctx context.Context, bServ blockservice.BlockService) error {
	shares := make([][]byte, appconsts.MinShareCount)
	for i := 0; i < appconsts.MinShareCount; i++ {
		shares[i] = tailPaddingShare
	}

	_, err := AddShares(ctx, shares, bServ)
	return err
}

// tail is filler for all tail padded shares
// it is allocated once and used everywhere
var tailPaddingShare = append(
	append(make([]byte, 0, appconsts.ShareSize), appconsts.TailPaddingNamespaceID...),
	bytes.Repeat([]byte{0}, appconsts.ShareSize-appconsts.NamespaceSize)...,
)
