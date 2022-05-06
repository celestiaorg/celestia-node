package ipld

import (
	"context"
	"math/rand"
	"testing"
	"time"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/da"
)

func TestRetriever_Retrieve(t *testing.T) {
	rand.Seed(time.Now().UnixNano()) // otherwise, the quadrant sampling is deterministic in tests

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dag := mdutils.Mock()
	r := NewRetriever(dag, DefaultRSMT2DCodec())

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
			shares := RandShares(t, tc.squareSize*tc.squareSize)
			in, err := AddShares(ctx, shares, dag)
			require.NoError(t, err)

			// limit with timeout, specifically retrieval
			ctx, cancel := context.WithTimeout(ctx, time.Minute*5) // the timeout is big for the max size which is long
			defer cancel()

			dah := da.NewDataAvailabilityHeader(in)
			out, err := r.Retrieve(ctx, &dah)
			require.NoError(t, err)
			assert.True(t, EqualEDS(in, out))
		})
	}
}
