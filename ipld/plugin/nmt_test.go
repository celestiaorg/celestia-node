package plugin

import (
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-core/pkg/da"
	"github.com/celestiaorg/celestia-core/testutils"
)

// TestNamespaceFromCID checks that deriving the Namespaced hash from
// the given CID works correctly.
func TestNamespaceFromCID(t *testing.T) {
	var tests = []struct {
		randData [][]byte
	}{
		{randData: testutils.GenerateRandNamespacedRawData(4, namespaceSize, ShareSize)},
		{randData: testutils.GenerateRandNamespacedRawData(16, 16, ShareSize)},
		{randData: testutils.GenerateRandNamespacedRawData(4, 4, ShareSize)},
		{randData: testutils.GenerateRandNamespacedRawData(4, namespaceSize, ShareSize/2)},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			// create DAH from rand data
			squareSize := uint64(math.Sqrt(float64(len(tt.randData))))
			dah, err := da.NewDataAvailabilityHeader(squareSize, tt.randData)
			require.NoError(t, err)
			// check to make sure NamespacedHash is correctly derived from CID
			for _, row := range dah.RowsRoots {
				c, err := CidFromNamespacedSha256(row)
				require.NoError(t, err)

				got := NamespacedSha256FromCID(c)
				assert.Equal(t, row, got)
			}
		})
	}
}
