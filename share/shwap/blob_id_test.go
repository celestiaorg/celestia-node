package shwap

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v3/share"
)

func TestBlobID_MarshalUnmarshal(t *testing.T) {
	ns := randomBlobNamespace(t)
	commitment := make([]byte, commitmentSize)
	_, err := rand.Read(commitment)
	require.NoError(t, err)

	id, err := NewBlobID(1, ns, commitment)
	require.NoError(t, err)

	data, err := id.MarshalBinary()
	require.NoError(t, err)
	require.Len(t, data, BlobIDSize)

	restored, err := BlobIDFromBinary(data)
	require.NoError(t, err)
	require.True(t, id.Equals(*restored))
}

func TestBlobID_ReadFromWriteTo(t *testing.T) {
	ns := randomBlobNamespace(t)
	commitment := make([]byte, commitmentSize)
	_, err := rand.Read(commitment)
	require.NoError(t, err)

	id, err := NewBlobID(42, ns, commitment)
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	n, err := id.WriteTo(buf)
	require.NoError(t, err)
	require.Equal(t, int64(BlobIDSize), n)

	var restored BlobID
	n, err = restored.ReadFrom(buf)
	require.NoError(t, err)
	require.Equal(t, int64(BlobIDSize), n)
	require.True(t, id.Equals(restored))
}

func TestBlobID_Validate(t *testing.T) {
	ns := randomBlobNamespace(t)
	commitment := make([]byte, commitmentSize)
	_, err := rand.Read(commitment)
	require.NoError(t, err)

	t.Run("zero height", func(t *testing.T) {
		_, err := NewBlobID(0, ns, commitment)
		require.Error(t, err)
	})

	t.Run("nil commitment", func(t *testing.T) {
		_, err := NewBlobID(1, ns, nil)
		require.Error(t, err)
	})

	t.Run("wrong commitment size", func(t *testing.T) {
		_, err := NewBlobID(1, ns, []byte{1, 2, 3})
		require.Error(t, err)
	})
}

func TestBlobsID_EmptyCommitment(t *testing.T) {
	ns := randomBlobNamespace(t)

	id, err := NewBlobsID(1, ns)
	require.NoError(t, err)
	require.True(t, bytes.Equal(id.Commitment, emptyCommitment))
}

func TestBlobIDFromBinary_InvalidLength(t *testing.T) {
	_, err := BlobIDFromBinary([]byte{1, 2, 3})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid BlobID length")
}

func randomBlobNamespace(t *testing.T) libshare.Namespace {
	t.Helper()
	nsID := make([]byte, libshare.NamespaceVersionZeroIDSize)
	_, err := rand.Read(nsID)
	require.NoError(t, err)
	// Ensure the first byte is non-zero to avoid reserved namespaces
	if nsID[0] == 0 {
		nsID[0] = 1
	}
	ns, err := libshare.NewV0Namespace(nsID)
	require.NoError(t, err)
	return ns
}
