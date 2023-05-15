package blob

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/celestia-app/testutil/testfactory"
	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"
)

func TestBlob(t *testing.T) {
	blob := generateBlob(t, []int{1}, false)

	var test = []struct {
		name        string
		expectedRes func(t *testing.T)
	}{
		{
			name: "new blob",
			expectedRes: func(t *testing.T) {
				require.NotEmpty(t, blob)
				require.NotEmpty(t, blob[0].NamespaceID())
				require.NotEmpty(t, blob[0].Data())
				require.NotEmpty(t, blob[0].Commitment())
			},
		},
		{
			name: "compare commitments",
			expectedRes: func(t *testing.T) {
				comm, err := apptypes.CreateCommitment(&blob[0].Blob)
				require.NoError(t, err)
				assert.Equal(t, blob[0].Commitment(), Commitment(comm))
			},
		},
		{
			name: "verify nID",
			expectedRes: func(t *testing.T) {
				require.NoError(t, apptypes.ValidateBlobNamespaceID(blob[0].NamespaceID()))
			},
		},
		{
			name: "shares to blobs",
			expectedRes: func(t *testing.T) {
				b, err := sharesToBlobs(splitBlob(t, blob...))
				require.NoError(t, err)
				assert.Equal(t, len(b), 1)
				assert.Equal(t, blob[0].Commitment(), b[0].Commitment())
			},
		},
	}

	for _, tt := range test {
		t.Run(tt.name, tt.expectedRes)
	}
}

func generateBlob(t *testing.T, sizes []int, sameNID bool) []*Blob {
	nID := tmrand.Bytes(appconsts.NamespaceSize)
	blobs := make([]*Blob, 0, len(sizes))

	for _, size := range sizes {
		size := rawBlobSize(appconsts.FirstSparseShareContentSize * size)
		appBlob := testfactory.GenerateRandomBlob(size)
		if sameNID {
			appBlob.NamespaceID = nID
		}

		blob, err := NewBlob(appBlob.ShareVersion, appBlob.NamespaceID, appBlob.Data)
		require.NoError(t, err)
		blobs = append(blobs, blob)
	}
	return blobs
}

func rawBlobSize(totalSize int) int {
	return totalSize - shares.DelimLen(uint64(totalSize))
}

func splitBlob(t *testing.T, blobs ...*Blob) [][]byte {
	t.Helper()
	tendermintBlobs := make([]types.Blob, 0)
	for _, blob := range blobs {
		tendermintBlobs = append(tendermintBlobs, types.Blob{
			NamespaceID:  blob.NamespaceID(),
			Data:         blob.Data(),
			ShareVersion: uint8(blob.Version()),
		})
	}
	sort.Slice(tendermintBlobs, func(i, j int) bool {
		val := bytes.Compare(tendermintBlobs[i].NamespaceID, tendermintBlobs[j].NamespaceID)
		return val <= 0
	})
	rawShares, err := shares.SplitBlobs(0, nil, tendermintBlobs, false)
	require.NoError(t, err)
	return shares.ToBytes(rawShares)
}
