package keystore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapKeystore(t *testing.T) {
	kstore := NewMapKeystore()
	err := kstore.Put("test", PrivKey{Body: []byte("test_private_key")})
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
