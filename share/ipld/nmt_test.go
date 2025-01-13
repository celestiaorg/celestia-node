package ipld

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

// TestNamespaceFromCID checks that deriving the Namespaced hash from
// the given CID works correctly.
func TestNamespaceFromCID(t *testing.T) {
	tests := []struct {
		eds *rsmt2d.ExtendedDataSquare
	}{
		// note that the number of shares must be a power of two
		{eds: edstest.RandEDS(t, 4)},
		{eds: edstest.RandEDS(t, 16)},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			roots, err := share.NewAxisRoots(tt.eds)
			require.NoError(t, err)
			// check to make sure NamespacedHash is correctly derived from CID
			for _, row := range roots.RowRoots {
				c, err := CidFromNamespacedSha256(row)
				require.NoError(t, err)

				got := NamespacedSha256FromCID(c)
				assert.Equal(t, row, got)
			}
		})
	}
}
