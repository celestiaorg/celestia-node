package bitswap

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestRangeNamespaceData_FetchRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()

	namespace := libshare.RandomNamespace()
	eds, root := edstest.RandEDSWithNamespace(t, namespace, 64, 8)
	exchange := newExchangeOverEDS(ctx, t, eds)

	testCases := []struct {
		name      string
		ns        libshare.Namespace
		from      shwap.SampleCoords
		to        shwap.SampleCoords
		expectErr bool
	}{
		{
			name:      "range fetch and verify",
			ns:        namespace,
			from:      shwap.SampleCoords{Row: 0, Col: 0},
			to:        shwap.SampleCoords{Row: 2, Col: 2},
			expectErr: false,
		},
		{
			name:      "single cell fetch and verify",
			ns:        namespace,
			from:      shwap.SampleCoords{Row: 1, Col: 1},
			to:        shwap.SampleCoords{Row: 1, Col: 1},
			expectErr: false,
		},
		{
			name:      "full row fetch and verify",
			ns:        namespace,
			from:      shwap.SampleCoords{Row: 0, Col: 0},
			to:        shwap.SampleCoords{Row: 0, Col: 7},
			expectErr: false,
		},
		{
			name:      "full column fetch and verify",
			ns:        namespace,
			from:      shwap.SampleCoords{Row: 0, Col: 3},
			to:        shwap.SampleCoords{Row: 7, Col: 3},
			expectErr: false,
		},
		{
			name:      "out of bounds row (should fail)",
			ns:        namespace,
			from:      shwap.SampleCoords{Row: 100, Col: 0},
			to:        shwap.SampleCoords{Row: 101, Col: 2},
			expectErr: true,
		},
		{
			name:      "out of bounds col (should fail)",
			ns:        namespace,
			from:      shwap.SampleCoords{Row: 0, Col: 100},
			to:        shwap.SampleCoords{Row: 2, Col: 101},
			expectErr: true,
		},
		{
			name:      "from greater than to (should fail)",
			ns:        namespace,
			from:      shwap.SampleCoords{Row: 3, Col: 3},
			to:        shwap.SampleCoords{Row: 2, Col: 2},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			blk, err := NewEmptyRangeNamespaceDataBlock(1, tc.ns, tc.from, tc.to, 16, false)
			if tc.expectErr {
				require.Error(t, err)
				return
			} else {
				require.NoError(t, err)
			}

			err = Fetch(ctx, exchange, root, []Block{blk})
			require.NoError(t, err)

			err = blk.Container.Verify(tc.ns, tc.from, tc.to, root.Hash())
			require.NoError(t, err)
		})
	}
}
