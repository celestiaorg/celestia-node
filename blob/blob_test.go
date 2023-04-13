package blob

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/celestia-app/testutil/testfactory"
	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"
)

func TestBlob_NewBlob(t *testing.T) {
	blob := generateBlob(t, 1)
	require.NotEmpty(t, blob)
}

func TestBlob_CompareCommitments(t *testing.T) {
	blob := generateBlob(t, 1)
	require.NotEmpty(t, blob.Commitment())

	comm, err := apptypes.CreateCommitment(&blob.Blob)
	require.NoError(t, err)
	assert.Equal(t, blob.Commitment(), Commitment(comm))
}

func TestBlob_NamespaceID(t *testing.T) {
	blob := generateBlob(t, 1)
	require.NotEmpty(t, blob)
	require.NotEmpty(t, blob.NamespaceID())
	require.NoError(t, apptypes.ValidateBlobNamespaceID(blob.NamespaceID()))
}

func TestBlob_Data(t *testing.T) {
	blob := generateBlob(t, 1)
	require.NotEmpty(t, blob)
	require.NotEmpty(t, blob.Data())
}

func TestSharesToBlobs(t *testing.T) {
	blob := generateBlob(t, 1)
	require.NotEmpty(t, blob)

	b, err := sharesToBlobs(splitBlob(t, blob))
	require.NoError(t, err)
	assert.Equal(t, len(b), 1)
	assert.Equal(t, blob.Commitment(), b[0].Commitment())
}

func generateBlob(t *testing.T, count int) *Blob {
	size := rawBlobSize(appconsts.FirstSparseShareContentSize * count)
	appBlob := testfactory.GenerateRandomBlob(size)
	blob, err := NewBlob(appBlob.ShareVersion, appBlob.NamespaceID, appBlob.Data)
	require.NoError(t, err)
	return blob
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
