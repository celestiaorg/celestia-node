package byzantine

import (
	"context"
	"strconv"
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
	var tests = []struct {
		roots [][]byte
	}{
		{dah.RowRoots},
		{dah.ColumnRoots},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			for j, root := range tt.roots {
				rootCid := ipld.MustCidFromNamespacedSha256(root)
				for index := 0; uint(index) < in.Width(); index++ {
					proof, err := getProofsAt(ctx, bServ, rootCid, index, int(in.Width()))
					require.NoError(t, err)
					node, err := ipld.GetLeaf(ctx, bServ, rootCid, index, int(in.Width()))
					require.NoError(t, err)
					inclusion := &ShareWithProof{
						Share: share.GetData(node.RawData()),
						Proof: &proof,
					}
					var x, y int
					if i == 0 {
						x, y = index, j
					} else {
						x, y = j, index
					}
					require.True(t, inclusion.Validate(rootCid, x, y, int(in.Width())))
				}
			}
		})
	}
}

func TestGetProofs(t *testing.T) {
	const width = 4
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	bServ := ipld.NewMemBlockservice()

	shares := sharetest.RandShares(t, width*width)
	in, err := ipld.AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	dah, err := da.NewDataAvailabilityHeader(in)
	require.NoError(t, err)
	for c, root := range dah.ColumnRoots {
		rootCid := ipld.MustCidFromNamespacedSha256(root)
		data := make([][]byte, 0, in.Width())
		for index := 0; uint(index) < in.Width(); index++ {
			node, err := ipld.GetLeaf(ctx, bServ, rootCid, index, int(in.Width()))
			require.NoError(t, err)
			data = append(data, share.GetData(node.RawData()))
		}

		proves, err := GetProofsForShares(ctx, bServ, rootCid, data, rsmt2d.Col)
		require.NoError(t, err)
		for i, proof := range proves {
			x, y := c, i
			require.True(t, proof.Validate(rootCid, x, y, int(in.Width())))
		}
	}
}
