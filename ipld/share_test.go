package ipld

import (
	"context"
	"testing"
	"time"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/da"
)

func TestBuildSharesFromPath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dag := mdutils.Mock()

	// generate EDS
	eds := generateRandEDS(t, 2)

	shares := ExtractODSShares(eds)

	in, err := PutData(ctx, shares, dag)
	require.NoError(t, err)

	// limit with deadline, specifically retrieval
	_ctx, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()
	dah := da.NewDataAvailabilityHeader(in)
	root := dah.RowsRoots[0]
	rootCid, err := plugin.CidFromNamespacedSha256(root)
	nd, _ := dag.Get(ctx, rootCid)
	require.Nil(t, err)

	leaves, err := getSubtreeLeavesWithProof(_ctx, nd.Cid(), dag, true)
	require.NoError(t, err)
	for _, leaf := range leaves {
		valid := leaf.Validate(root)
		require.True(t, valid)
	}
}

func TestBuildSharesFromLeaves(t *testing.T) {
	eds := RandEDS(t, 2)
	type test struct {
		name        string
		length      int
		errShares   func(uint) [][]byte
		dataFetcher func(uint) [][]byte
		roots       [][]byte
	}
	var tests = []test{
		{
			name:        "build tree from row roots",
			length:      len(eds.RowRoots()),
			errShares:   eds.Row,
			dataFetcher: eds.Col,
			roots:       eds.ColRoots(),
		},
		{
			name:        "build tree from col roots",
			length:      len(eds.ColRoots()),
			errShares:   eds.Col,
			dataFetcher: eds.Row,
			roots:       eds.RowRoots(),
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i < len(tests); i++ {
				errShares := tc.errShares(uint(i))
				for index := range errShares {
					isParityShare := false
					if i >= tc.length/2 || index >= tc.length/2 {
						isParityShare = true
					}
					shares, err := NewShareWithProofFromLeaves(tc.dataFetcher(uint(index)), tc.roots[index], uint(index), i, isParityShare)
					require.Nil(t, err)
					require.True(t, shares.Validate(tc.roots[index]))
				}
			}
		})
	}

}
