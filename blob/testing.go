package blob

import (
	"testing"

	"github.com/stretchr/testify/require"
	tmrand "github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/celestia-app/testutil/testfactory"
)

func GenerateBlobs(t *testing.T, sizes []int, sameNID bool) []*Blob {
	nID := tmrand.Bytes(appconsts.NamespaceSize)
	blobs := make([]*Blob, 0, len(sizes))

	for _, size := range sizes {
		size := rawBlobSize(t, appconsts.FirstSparseShareContentSize*size)
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

func rawBlobSize(t *testing.T, totalSize int) int {
	t.Helper()
	return totalSize - shares.DelimLen(uint64(totalSize))
}
