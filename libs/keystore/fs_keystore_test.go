package keystore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSKeystore(t *testing.T) {
	kstore, err := NewFSKeystore(t.TempDir()+"/keystore", nil)
	require.NoError(t, err)

	err = kstore.Put("test", PrivKey{Body: []byte("test_private_key")})
	require.NoError(t, err)

	key, err := kstore.Get("test")
	require.NoError(t, err)
	assert.Equal(t, []byte("test_private_key"), key.Body)

	keys, err := kstore.List()
	require.NoError(t, err)
	assert.Len(t, keys, 1)

	err = kstore.Delete("test")
	require.NoError(t, err)

	keys, err = kstore.List()
	require.NoError(t, err)
	assert.Len(t, keys, 0)
}
