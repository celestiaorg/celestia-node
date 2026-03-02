package bitswap

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v3/share"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestRangeNamespaceData_FetchRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	namespace := libshare.RandomNamespace()
	eds, root := edstest.RandEDSWithNamespace(t, namespace, 64, 8)
	exchange := newExchangeOverEDS(ctx, t, eds)

	testCases := []struct {
		name      string
		ns        libshare.Namespace
		from      int
		to        int
		expectErr bool
	}{
		{
			name:      "range fetch and verify",
			ns:        namespace,
			from:      0,
			to:        18,
			expectErr: false,
		},
		{
			name:      "single cell fetch and verify",
			ns:        namespace,
			from:      9,
			to:        10,
			expectErr: false,
		},
		{
			name:      "full row fetch and verify",
			ns:        namespace,
			from:      0,
			to:        8,
			expectErr: false,
		},
		{
			name:      "full ods fetch and verify",
			ns:        namespace,
			from:      0,
			to:        64,
			expectErr: false,
		},
		{
			name:      "out of bounds start (should fail)",
			ns:        namespace,
			from:      100,
			to:        115,
			expectErr: true,
		},
		{
			name:      "out of bounds end (should fail)",
			ns:        namespace,
			from:      1,
			to:        100,
			expectErr: true,
		},
		{
			name:      "from greater than to (should fail)",
			ns:        namespace,
			from:      12,
			to:        5,
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			blk, err := NewEmptyRangeNamespaceDataBlock(1, tc.from, tc.to, 8)
			if tc.expectErr {
				require.Error(t, err)
				return
			} else {
				require.NoError(t, err)
			}

			err = Fetch(ctx, exchange, root, []Block{blk})
			require.NoError(t, err)
			from, err := shwap.SampleCoordsFrom1DIndex(tc.from, 8)
			require.NoError(t, err)
			to, err := shwap.SampleCoordsFrom1DIndex(tc.to-1, 8)
			require.NoError(t, err)
			err = blk.Container.VerifyInclusion(from, to, len(root.RowRoots)/2, root.RowRoots[from.Row:to.Row+1])
			require.NoError(t, err)
		})
	}
}
