package blob

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v2/pkg/appconsts"
	squareblob "github.com/celestiaorg/go-square/blob"
	"github.com/celestiaorg/go-square/namespace"
)

func TestBlobsToShares(t *testing.T) {
	t.Run("should sort blobs by namespace in ascending order", func(t *testing.T) {
		namespaceA := namespace.MustNewV0(bytes.Repeat([]byte{0xa}, 10)).Bytes()
		namespaceB := namespace.MustNewV0(bytes.Repeat([]byte{0xb}, 10)).Bytes()

		blobA, err := NewBlob(appconsts.ShareVersionZero, namespaceA, []byte("dataA"))
		require.NoError(t, err)
		blobB, err := NewBlob(appconsts.ShareVersionZero, namespaceB, []byte("dataB"))
		require.NoError(t, err)

		got, err := BlobsToShares(blobB, blobA)
		require.NoError(t, err)
		assert.Equal(t, got[0][:appconsts.NamespaceSize], namespaceA)
		assert.Equal(t, got[1][:appconsts.NamespaceSize], namespaceB)
		assert.IsIncreasing(t, got)
	})
}

func TestToAppBlobs(t *testing.T) {
	namespaceA := namespace.MustNewV0(bytes.Repeat([]byte{0xa}, 10))
	namespaceB := namespace.MustNewV0(bytes.Repeat([]byte{0xb}, 10))

	blobA, err := NewBlob(appconsts.ShareVersionZero, namespaceA.Bytes(), []byte("dataA"))
	require.NoError(t, err)
	blobB, err := NewBlob(appconsts.ShareVersionZero, namespaceB.Bytes(), []byte("dataB"))
	require.NoError(t, err)

	got := ToAppBlobs(blobA, blobB)
	want := []*squareblob.Blob{
		{
			NamespaceId:      blobA.NamespaceId,
			NamespaceVersion: blobA.NamespaceVersion,
			Data:             blobA.Data,
			ShareVersion:     blobA.ShareVersion,
		},
		{
			NamespaceId:      blobB.NamespaceId,
			NamespaceVersion: blobB.NamespaceVersion,
			Data:             blobB.Data,
			ShareVersion:     blobB.ShareVersion,
		},
	}
	assert.Equal(t, want, got)
}
