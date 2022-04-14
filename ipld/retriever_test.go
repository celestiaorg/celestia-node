package ipld

import (
	"context"
	"testing"
	"time"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/da"

	"github.com/celestiaorg/rsmt2d"
)

func TestRetriever_Retrieve(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dag := mdutils.Mock()
	r := NewRetriever(dag, rsmt2d.NewRSGF8Codec())

	type test struct {
		name       string
		squareSize int
	}
	tests := []test{
		{"1x1(min)", 1},
		{"2x2(med)", 2},
		{"32x32(med)", 32},
		{"128x128(max)", MaxSquareSize},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// generate EDS
			shares := RandNamespacedShares(t, tc.squareSize*tc.squareSize)
			in, err := PutData(ctx, shares.Raw(), dag)
			require.NoError(t, err)

			// limit with timeout, specifically retrieval
			ctx, cancel := context.WithTimeout(ctx, time.Second*30)
			defer cancel()

			dah := da.NewDataAvailabilityHeader(in)
			out, err := r.Retrieve(ctx, &dah)
			require.NoError(t, err)
			assert.True(t, EqualEDS(in, out))
		})
	}
}
