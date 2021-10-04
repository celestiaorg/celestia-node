package ipld

import (
	"context"
	"fmt"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-core/pkg/wrapper"
)

// PutData posts erasured block data to IPFS using the provided ipld.NodeAdder.
func PutData(ctx context.Context, eds *rsmt2d.ExtendedDataSquare, adder ipld.NodeAdder) (*rsmt2d.ExtendedDataSquare, error) {
	// create nmt adder wrapping batch adder
	batchAdder := NewNmtNodeAdder(ctx, ipld.NewBatch(ctx, adder))
	// create the nmt wrapper to generate row and col commitments
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(eds.Width()), nmt.NodeVisitor(batchAdder.Visit))
	// recompute the eds // TODO @renaynay: is this right?
	shares := make([][]byte, 0)
	for i := 0; i < int(eds.Width()/2); i++ {
		shares = append(shares, eds.Row(uint(i))...)
	}
	recomputed, err := rsmt2d.ComputeExtendedDataSquare(shares, rsmt2d.NewRSGF8Codec(), tree.Constructor)
	if err != nil {
		return nil, fmt.Errorf("failure to recompute the extended data square: %w", err)
	}
	// compute roots
	recomputed.ColRoots()
	// commit the batch to ipfs
	return eds, batchAdder.Commit()
}
