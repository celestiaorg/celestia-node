package byzantine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestGetProof(t *testing.T) {
	const width = 4

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	bServ := ipld.NewMemBlockservice()

	shares := sharetest.RandShares(t, width*width)
	in, err := ipld.AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	dah, err := da.NewDataAvailabilityHeader(in)
	require.NoError(t, err)

	for _, axisType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
		var roots [][]byte
		switch axisType {
		case rsmt2d.Row:
			roots = dah.RowRoots
		case rsmt2d.Col:
			roots = dah.ColumnRoots
		}
		for axisIdx := 0; axisIdx < width; axisIdx++ {
			rootCid := ipld.MustCidFromNamespacedSha256(roots[axisIdx])
			for shrIdx := 0; shrIdx < width; shrIdx++ {
				proof, err := getProofsAt(ctx, bServ, rootCid, shrIdx, int(in.Width()))
				require.NoError(t, err)
				node, err := ipld.GetLeaf(ctx, bServ, rootCid, shrIdx, int(in.Width()))
				require.NoError(t, err)
				inclusion := &ShareWithProof{
					Share: share.GetData(node.RawData()),
					Proof: &proof,
					Axis:  axisType,
				}
				require.True(t, inclusion.Validate(&dah, axisType, axisIdx, shrIdx))
			}
		}
	}
}
