package ipld

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/sharetest"
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
	var tests = []struct {
		roots    [][]byte
		axisType rsmt2d.Axis
	}{
		{
			roots:    dah.RowRoots,
			axisType: rsmt2d.Row,
		},
		{
			roots:    dah.ColumnRoots,
			axisType: rsmt2d.Col,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			for axisIdx, root := range tt.roots {
				for shrIdx := 0; uint(shrIdx) < in.Width(); shrIdx++ {
					share, err := GetShareWithProof(ctx, bServ, root, shrIdx, int(in.Width()), tt.axisType)
					require.NoError(t, err)
					require.True(t, share.Validate(root, shrIdx, axisIdx, int(in.Width())))
				}
			}
		})
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
			require.True(t, proof.Validate(root, i, axisIdx, int(in.Width())))
		}
	}
}
