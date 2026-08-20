package node

import (
	"fmt"
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

type recordingSink struct {
	mu    sync.Mutex
	calls []string
}

func (r *recordingSink) OnRevoke(nonceHex string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, nonceHex)
}

func TestRevoker_NotifiesSinksOnRevoke(t *testing.T) {
	r, err := NewRevoker(filepath.Join(t.TempDir(), "revoked.json"))
	require.NoError(t, err)

	sink := &recordingSink{}
	r.AddSink(sink)

	require.NoError(t, r.Revoke([]byte{0xde, 0xad, 0xbe, 0xef}))
	// Idempotent revoke of the same nonce must not fire the sink again.
	require.NoError(t, r.Revoke([]byte{0xde, 0xad, 0xbe, 0xef}))

	sink.mu.Lock()
	defer sink.mu.Unlock()
	assert.Equal(t, []string{"deadbeef"}, sink.calls)
}

func TestRevoker_ConcurrentRevoke(t *testing.T) {
	r, err := NewRevoker(filepath.Join(t.TempDir(), "revoked.json"))
	require.NoError(t, err)

	const workers = 8
	const each = 25
	errs := make(chan error, workers*each)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				nonce := []byte{byte(w), byte(i)}
				if err := r.Revoke(nonce); err != nil {
					errs <- err
					return
				}
				if !r.IsRevoked(nonce) {
					errs <- fmt.Errorf("nonce %x not marked revoked", nonce)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	assert.Len(t, r.List(), workers*each)

	r2, err := NewRevoker(r.path)
	require.NoError(t, err)
	assert.Len(t, r2.List(), workers*each)
}
