package blob

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gosquare "github.com/celestiaorg/go-square/v2/share"
)

func TestBlobsToShares(t *testing.T) {
	t.Run("should sort blobs by namespace in ascending order", func(t *testing.T) {
		namespaceA := gosquare.MustNewV0Namespace(bytes.Repeat([]byte{0xa}, 10))
		namespaceB := gosquare.MustNewV0Namespace(bytes.Repeat([]byte{0xb}, 10))

		blobA, err := NewBlob(gosquare.ShareVersionZero, namespaceA, []byte("dataA"), nil)
		require.NoError(t, err)
		blobB, err := NewBlob(gosquare.ShareVersionZero, namespaceB, []byte("dataB"), nil)
		require.NoError(t, err)

		got, err := BlobsToShares(blobB, blobA)
		require.NoError(t, err)
		assert.Equal(t, got[0].Namespace(), namespaceA)
		assert.Equal(t, got[1].Namespace(), namespaceB)
		assert.True(t, got[0].Namespace().IsLessThan(got[1].Namespace()))
	})
}

func TestToAppBlobs(t *testing.T) {
	namespaceA := gosquare.MustNewV0Namespace(bytes.Repeat([]byte{0xa}, 10))
	namespaceB := gosquare.MustNewV0Namespace(bytes.Repeat([]byte{0xb}, 10))

	blobA, err := NewBlob(gosquare.ShareVersionZero, namespaceA, []byte("dataA"), nil)
	require.NoError(t, err)
	blobB, err := NewBlob(gosquare.ShareVersionZero, namespaceB, []byte("dataB"), nil)
	require.NoError(t, err)

	got := ToAppBlobs(blobA, blobB)

	assert.Equal(t, []*gosquare.Blob{blobA.Blob, blobB.Blob}, got)
}
