package ipld

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/testing/sharetest"
)

func TestGetProof(t *testing.T) {
	const width = 4

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	bServ := NewMemBlockservice()

	shares := sharetest.RandShares(t, width*width)
	in, err := AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	dah, err := da.NewDataAvailabilityHeader(in)
	require.NoError(t, err)

	for _, axisType := range []rsmt2d.Axis{rsmt2d.Row, rsmt2d.Col} {
		for x := 0; uint(x) < in.Width(); x++ {
			for y := 0; uint(y) < in.Width(); y++ {
				index, rootCid := x, dah.RowRoots[y]
				if axisType == rsmt2d.Col {
					index, rootCid = y, dah.ColumnRoots[x]
				}
				share, err := GetShareWithProof(ctx, bServ, rootCid, index, int(in.Width()), axisType)
				require.NoError(t, err)
				require.True(t, share.VerifyInclusion(&dah, x, y))
			}
		}
	}
}

func TestGetSharesProofs(t *testing.T) {
	const width = 4
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	bServ := NewMemBlockservice()

	shares := sharetest.RandShares(t, width*width)
	in, err := AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	dah, err := da.NewDataAvailabilityHeader(in)
	require.NoError(t, err)
	for axisIdx, root := range dah.ColumnRoots {
		rootCid := MustCidFromNamespacedSha256(root)
		data := make([][]byte, 0, in.Width())
		for index := 0; uint(index) < in.Width(); index++ {
			node, err := GetLeaf(ctx, bServ, rootCid, index, int(in.Width()))
			require.NoError(t, err)
			data = append(data, node.RawData()[9:])
		}

		proves, err := GetSharesWithProofs(ctx, bServ, root, data, rsmt2d.Col)
		require.NoError(t, err)
		for i, proof := range proves {
			require.True(t, proof.VerifyInclusion(&dah, i, axisIdx))
		}
	}
}
