package ipld

import (
	"bytes"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/consts"

	"github.com/tendermint/tendermint/pkg/da"
)

// TestNamespaceFromCID checks that deriving the Namespaced hash from
// the given CID works correctly.
func TestNamespaceFromCID(t *testing.T) {
	var tests = []struct {
		randData [][]byte
	}{
		{randData: generateRandNamespacedRawData(4, consts.NamespaceSize, consts.ShareSize)},
		{randData: generateRandNamespacedRawData(16, 16, consts.ShareSize)},
		{randData: generateRandNamespacedRawData(4, 4, consts.ShareSize)},
		{randData: generateRandNamespacedRawData(4, consts.NamespaceSize, consts.ShareSize/2)},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			// create DAH from rand data
			squareSize := uint64(math.Sqrt(float64(len(tt.randData))))
			eds, err := da.ExtendShares(squareSize, tt.randData)
			require.NoError(t, err)
			dah := da.NewDataAvailabilityHeader(eds)
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

// generateRandNamespacedRawData returns random namespaced raw data for testing purposes.
func generateRandNamespacedRawData(total, nidSize, leafSize uint32) [][]byte {
	data := make([][]byte, total)
	for i := uint32(0); i < total; i++ {
		nid := make([]byte, nidSize)

		rand.Read(nid)
		data[i] = nid
	}
	sortByteArrays(data)
	for i := uint32(0); i < total; i++ {
		d := make([]byte, leafSize)

		rand.Read(d)
		data[i] = append(data[i], d...)
	}

	return data
}

func sortByteArrays(src [][]byte) {
	sort.Slice(src, func(i, j int) bool { return bytes.Compare(src[i], src[j]) < 0 })
}
