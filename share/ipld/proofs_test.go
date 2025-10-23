package ipld

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestGetProof(t *testing.T) {
	const width = 8

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	bServ := NewMemBlockservice()

	shares, err := libshare.RandShares(width * width)
	require.NoError(t, err)
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
		for axisIdx := range width * 2 {
			root := roots[axisIdx]
			for shrIdx := range width * 2 {
				proof, err := GetProof(ctx, bServ, root, shrIdx, int(in.Width()))
				require.NoError(t, err)
				rootCid := MustCidFromNamespacedSha256(root)
				node, err := GetLeaf(ctx, bServ, rootCid, shrIdx, int(in.Width()))
				require.NoError(t, err)

				sh, err := libshare.NewShare(node.RawData()[libshare.NamespaceSize:])
				require.NoError(t, err)
				sample := shwap.Sample{
					Share:     *sh,
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
				err = sample.Verify(axisRoots, rowIdx, colIdx)
				require.NoError(t, err)
			}
		}
	}
}
