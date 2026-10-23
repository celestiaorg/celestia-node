package authtoken

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cristalhq/jwt/v5"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	badger "github.com/ipfs/go-ds-badger4"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/api/rpc/perms"
)

func testSigner(t *testing.T) (jwt.Signer, jwt.Verifier) {
	t.Helper()
	key := []byte("test-secret-key-for-jwt-signing-32")
	signer, err := jwt.NewSignerHS(jwt.HS256, key)
	require.NoError(t, err)
	verifier, err := jwt.NewVerifierHS(jwt.HS256, key)
	require.NoError(t, err)
	return signer, verifier
}

func TestRevocationPersists(t *testing.T) {
	ctx := context.Background()
	signer, verifier := testSigner(t)
	path := t.TempDir()
	store, err := badger.NewDatastore(path, nil)
	require.NoError(t, err)
	v := NewVerifier(verifier, store)
	token, err := NewSignedJWT(signer, perms.AllPerms, 0)
	require.NoError(t, err)
	other, err := NewSignedJWT(signer, perms.AllPerms, time.Hour)
	require.NoError(t, err)
	// Older tokens have no nonce or expiry.
	legacy, err := jwt.NewBuilder(signer).Build(map[string]any{"Allow": perms.ReadPerms})
	require.NoError(t, err)
	for _, token := range []string{token, legacy.String()} {
		_, err = v.Verify(ctx, token)
		require.NoError(t, err)
		require.NoError(t, v.Revoke(ctx, token))
		require.NoError(t, v.Revoke(ctx, token))
		_, err = v.Verify(ctx, token)
		require.ErrorIs(t, err, ErrRevoked)
	}
	require.NoError(t, store.Close())
	store, err = badger.NewDatastore(path, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	v = NewVerifier(verifier, store)
	for _, token := range []string{token, legacy.String()} {
		_, err = v.Verify(ctx, token)
		require.ErrorIs(t, err, ErrRevoked)
	}
	allowed, err := v.Verify(ctx, other)
	require.NoError(t, err)
	require.Equal(t, perms.AllPerms, allowed)
}

func TestRevocationRejectsInvalidTokens(t *testing.T) {
	ctx := context.Background()
	signer, verifier := testSigner(t)
	v := NewVerifier(verifier, ds_sync.MutexWrap(datastore.NewMapDatastore()))
	foreignSigner, err := jwt.NewSignerHS(jwt.HS256, []byte("another-secret-key-for-signing-32"))
	require.NoError(t, err)
	foreign, err := NewSignedJWT(foreignSigner, perms.AllPerms, 0)
	require.NoError(t, err)
	for _, token := range []string{"", "invalid", foreign} {
		require.Error(t, v.Revoke(ctx, token))
		_, err := v.Verify(ctx, token)
		require.Error(t, err)
	}
	expired, err := NewSignedJWT(signer, perms.AllPerms, -time.Hour)
	require.NoError(t, err)
	_, err = v.Verify(ctx, expired)
	require.ErrorContains(t, err, "token expired")
	require.NoError(t, v.Revoke(ctx, expired))
}

func TestRevocationAlternateSignatureEncoding(t *testing.T) {
	ctx := context.Background()
	signer, verifier := testSigner(t)
	v := NewVerifier(verifier, ds_sync.MutexWrap(datastore.NewMapDatastore()))
	token, err := NewSignedJWT(signer, perms.AllPerms, 0)
	require.NoError(t, err)
	// HS256 leaves two unused bits in the last base64 character.
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	last := strings.IndexByte(alphabet, token[len(token)-1])
	alternate := token[:len(token)-1] + string(alphabet[last+1])
	_, err = v.Verify(ctx, alternate)
	require.NoError(t, err)
	require.NoError(t, v.Revoke(ctx, token))
	_, err = v.Verify(ctx, alternate)
	require.ErrorIs(t, err, ErrRevoked)
}

type failingStore struct {
	datastore.Datastore
	hasErr, putErr, syncErr error
}

func (s failingStore) Has(ctx context.Context, key datastore.Key) (bool, error) {
	if s.hasErr != nil {
		return false, s.hasErr
	}
	return s.Datastore.Has(ctx, key)
}

func (s failingStore) Put(ctx context.Context, key datastore.Key, value []byte) error {
	if s.putErr != nil {
		return s.putErr
	}
	return s.Datastore.Put(ctx, key, value)
}

func (s failingStore) Sync(ctx context.Context, key datastore.Key) error {
	if s.syncErr != nil {
		return s.syncErr
	}
	return s.Datastore.Sync(ctx, key)
}

func TestRevocationStoreErrors(t *testing.T) {
	signer, verifier := testSigner(t)
	token, err := NewSignedJWT(signer, perms.AllPerms, 0)
	require.NoError(t, err)
	failure := errors.New("store unavailable")
	for _, operation := range []string{"read", "write", "sync"} {
		t.Run(operation, func(t *testing.T) {
			store := &failingStore{Datastore: ds_sync.MutexWrap(datastore.NewMapDatastore())}
			v := NewVerifier(verifier, store)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			release, err := v.Register(ctx, token, cancel)
			require.NoError(t, err)
			defer release()
			switch operation {
			case "read":
				store.hasErr = failure
				_, err = v.Verify(ctx, token)
				require.ErrorIs(t, err, failure)
				_, err = v.Register(ctx, token, cancel)
				require.ErrorIs(t, err, failure)
			case "write":
				store.putErr = failure
				require.ErrorIs(t, v.Revoke(ctx, token), failure)
				require.NoError(t, ctx.Err())
				_, err = v.Verify(ctx, token)
				require.NoError(t, err)
			case "sync":
				store.syncErr = failure
				require.ErrorIs(t, v.Revoke(ctx, token), failure)
				require.ErrorIs(t, ctx.Err(), context.Canceled)
				_, err = v.Verify(context.Background(), token)
				require.ErrorIs(t, err, ErrRevoked)
			}
		})
	}
}

func TestRevokeDuringRegistration(t *testing.T) {
	signer, verifier := testSigner(t)
	v := NewVerifier(verifier, ds_sync.MutexWrap(datastore.NewMapDatastore()))
	for range 50 {
		token, err := NewSignedJWT(signer, perms.AllPerms, 0)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		var revokeErr error
		go func() {
			defer wg.Done()
			<-start
			revokeErr = v.Revoke(context.Background(), token)
		}()
		close(start)
		release, err := v.Register(ctx, token, cancel)
		wg.Wait()
		require.NoError(t, revokeErr)
		if err == nil {
			require.ErrorIs(t, ctx.Err(), context.Canceled)
			release()
		} else {
			require.ErrorIs(t, err, ErrRevoked)
		}
		cancel()
	}
	require.Empty(t, v.sessions)
}

func TestRegistrationRelease(t *testing.T) {
	signer, verifier := testSigner(t)
	v := NewVerifier(verifier, ds_sync.MutexWrap(datastore.NewMapDatastore()))
	token, err := NewSignedJWT(signer, perms.AllPerms, 0)
	require.NoError(t, err)
	first, cancelFirst := context.WithCancel(context.Background())
	defer cancelFirst()
	second, cancelSecond := context.WithCancel(context.Background())
	defer cancelSecond()
	releaseFirst, err := v.Register(first, token, cancelFirst)
	require.NoError(t, err)
	releaseSecond, err := v.Register(second, token, cancelSecond)
	require.NoError(t, err)
	defer releaseSecond()
	releaseFirst()
	require.NoError(t, v.Revoke(context.Background(), token))
	require.NoError(t, first.Err(), "finished requests must be unregistered")
	require.ErrorIs(t, second.Err(), context.Canceled)
	require.Empty(t, v.sessions)
}
