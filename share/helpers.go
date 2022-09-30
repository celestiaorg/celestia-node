package share

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
)

// job represents an encountered node to investigate during the `share.GetShares`
// and `share.GetSharesByNamespace` routines.
type job struct {
	id  cid.Cid
	pos int
	ctx context.Context
}

func SanityCheckNID(nID []byte) error {
	if len(nID) != NamespaceSize {
		return fmt.Errorf("expected namespace ID of size %d, got %d", NamespaceSize, len(nID))
	}
	return nil
}

// BatchSize calculates the amount of nodes that are generated from block of 'squareSizes'
// to be batched in one write.
func BatchSize(squareSize int) int {
	// (squareSize*2-1) - amount of nodes in a generated binary tree
	// squareSize*2 - the total number of trees, both for rows and cols
	// (squareSize*squareSize) - all the shares
	//
	// Note that while our IPLD tree looks like this:
	// ---X
	// -X---X
	// X-X-X-X
	// X-X-X-X
	// here we count leaves only once: the CIDs are the same for columns and rows
	// and for the last two layers as well:
	return (squareSize*2-1)*squareSize*2 - (squareSize * squareSize)
}
