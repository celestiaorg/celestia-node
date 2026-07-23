package node

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRevoker_RevokeAndCheck(t *testing.T) {
	r, err := NewRevoker(filepath.Join(t.TempDir(), "revoked.json"))
	require.NoError(t, err)

	nonce := []byte("nonce-a")
	assert.False(t, r.IsRevoked(nonce))
	require.NoError(t, r.Revoke(nonce))
	assert.True(t, r.IsRevoked(nonce))
}

func TestRevoker_RevokeIsIdempotent(t *testing.T) {
	r, err := NewRevoker(filepath.Join(t.TempDir(), "revoked.json"))
	require.NoError(t, err)

	nonce := []byte("nonce-a")
	require.NoError(t, r.Revoke(nonce))
	require.NoError(t, r.Revoke(nonce))
	assert.Len(t, r.List(), 1)
}

func TestRevoker_EmptyNonceRejected(t *testing.T) {
	r, err := NewRevoker(filepath.Join(t.TempDir(), "revoked.json"))
	require.NoError(t, err)

	assert.Error(t, r.Revoke(nil))
	assert.False(t, r.IsRevoked(nil))
}

func TestRevoker_PersistsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "revoked.json")
	r1, err := NewRevoker(path)
	require.NoError(t, err)
	require.NoError(t, r1.Revoke([]byte("compromised")))

	r2, err := NewRevoker(path)
	require.NoError(t, err)
	assert.True(t, r2.IsRevoked([]byte("compromised")))
}

func TestRevoker_ConcurrentRevoke(t *testing.T) {
	r, err := NewRevoker(filepath.Join(t.TempDir(), "revoked.json"))
	require.NoError(t, err)

	const workers = 8
	const each = 25
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				nonce := []byte{byte(w), byte(i)}
				require.NoError(t, r.Revoke(nonce))
				assert.True(t, r.IsRevoked(nonce))
			}
		}(w)
	}
	wg.Wait()
	assert.Len(t, r.List(), workers*each)

	r2, err := NewRevoker(r.path)
	require.NoError(t, err)
	assert.Len(t, r2.List(), workers*each)
}
