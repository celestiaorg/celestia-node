package ipld

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/sharetest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestGetProof(t *testing.T) {
	const width = 8

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	bServ := NewMemBlockservice()

	shares := sharetest.RandShares(t, width*width)
	in, err := AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	axisRoots, err := share.NewAxisRoots(in)
	require.NoError(t, err)

	for _, proofType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
		var roots [][]byte
		switch proofType {
		case rsmt2d.Row:
			roots = axisRoots.RowRoots
		case rsmt2d.Col:
			roots = axisRoots.ColumnRoots
		}
		for axisIdx := 0; axisIdx < width*2; axisIdx++ {
			root := roots[axisIdx]
			for shrIdx := 0; shrIdx < width*2; shrIdx++ {
				proof, err := GetProof(ctx, bServ, root, shrIdx, int(in.Width()))
				require.NoError(t, err)
				rootCid := MustCidFromNamespacedSha256(root)
				node, err := GetLeaf(ctx, bServ, rootCid, shrIdx, int(in.Width()))
				require.NoError(t, err)

				sample := shwap.Sample{
					Share:     share.GetData(node.RawData()),
					Proof:     &proof,
					ProofType: proofType,
				}
				var rowIdx, colIdx int
				switch proofType {
				case rsmt2d.Row:
					rowIdx, colIdx = axisIdx, shrIdx
				case rsmt2d.Col:
					rowIdx, colIdx = shrIdx, axisIdx
				}
				err = sample.Validate(axisRoots, rowIdx, colIdx)
				require.NoError(t, err)
			}
		}
	}
}
