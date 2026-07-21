package node

import (
	"encoding/hex"
	"encoding/json"
	"os"
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
	assert.Error(t, r.Revoke([]byte{}))
	assert.False(t, r.IsRevoked(nil))
	assert.False(t, r.IsRevoked([]byte{}))
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

func TestRevoker_ListSortedHex(t *testing.T) {
	r, err := NewRevoker(filepath.Join(t.TempDir(), "revoked.json"))
	require.NoError(t, err)

	require.NoError(t, r.Revoke([]byte{0xff}))
	require.NoError(t, r.Revoke([]byte{0x01}))
	require.NoError(t, r.Revoke([]byte{0x80}))

	got := r.List()
	assert.Equal(t, []string{
		hex.EncodeToString([]byte{0x01}),
		hex.EncodeToString([]byte{0x80}),
		hex.EncodeToString([]byte{0xff}),
	}, got)
}

func TestRevoker_LoadMissingFileIsEmpty(t *testing.T) {
	r, err := NewRevoker(filepath.Join(t.TempDir(), "does-not-exist.json"))
	require.NoError(t, err)
	assert.Empty(t, r.List())
}

func TestRevoker_LoadCorruptFileErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "revoked.json")
	require.NoError(t, os.WriteFile(path, []byte("not-json"), 0o600))

	_, err := NewRevoker(path)
	assert.Error(t, err)
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

func TestRevoker_MergesExternalWritesOnNextRevoke(t *testing.T) {
	path := filepath.Join(t.TempDir(), "revoked.json")
	r, err := NewRevoker(path)
	require.NoError(t, err)
	require.NoError(t, r.Revoke([]byte("in-memory")))

	// Simulate the offline CLI writing directly to disk while the node holds an in-memory set.
	external := []string{
		hex.EncodeToString([]byte("in-memory")),
		hex.EncodeToString([]byte("cli-added")),
	}
	data, err := json.Marshal(external)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))

	// The next RPC-triggered revoke must merge, not overwrite.
	require.NoError(t, r.Revoke([]byte("rpc-added")))

	r2, err := NewRevoker(path)
	require.NoError(t, err)
	assert.True(t, r2.IsRevoked([]byte("in-memory")))
	assert.True(t, r2.IsRevoked([]byte("cli-added")))
	assert.True(t, r2.IsRevoked([]byte("rpc-added")))
}

func TestRevoker_OnDiskFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "revoked.json")
	r, err := NewRevoker(path)
	require.NoError(t, err)
	require.NoError(t, r.Revoke([]byte{0xde, 0xad}))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var got []string
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, []string{"dead"}, got)
}
