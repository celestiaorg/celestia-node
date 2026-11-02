package authtoken

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/cristalhq/jwt/v5"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/ipfs/go-datastore"
)

// ErrRevoked indicates that a token has been revoked.
var ErrRevoked = errors.New("token revoked")

// Verifier checks JWT signatures, expiry and persistent revocations.
// Active requests are canceled when their token is revoked.
type Verifier struct {
	verifier jwt.Verifier
	store    datastore.Datastore
	mu       sync.Mutex
	sessions map[datastore.Key]map[context.Context]context.CancelFunc
}

// NewVerifier uses the node's datastore to persist revocations across restarts.
func NewVerifier(verifier jwt.Verifier, store datastore.Datastore) *Verifier {
	return &Verifier{
		verifier: verifier,
		store:    store,
		sessions: make(map[datastore.Key]map[context.Context]context.CancelFunc),
	}
}

// Verify returns the permissions of a valid, unrevoked token.
func (v *Verifier) Verify(ctx context.Context, token string) ([]auth.Permission, error) {
	tk, err := jwt.Parse([]byte(token), v.verifier)
	if err != nil {
		return nil, err
	}
	return v.verify(ctx, tk)
}

// VerifyAndRegister verifies a token and cancels the request when it is revoked.
// Call the returned function when the request finishes. The context must be unique per request.
func (v *Verifier) VerifyAndRegister(
	ctx context.Context, token string, cancel context.CancelFunc,
) ([]auth.Permission, func(), error) {
	tk, err := jwt.Parse([]byte(token), v.verifier)
	if err != nil {
		return nil, nil, err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	// Checking under the same lock as Revoke closes the authentication/registration race.
	permissions, err := v.verify(ctx, tk)
	if err != nil {
		return nil, nil, err
	}
	key := tokenKey(tk)
	if v.sessions[key] == nil {
		v.sessions[key] = make(map[context.Context]context.CancelFunc)
	}
	v.sessions[key][ctx] = cancel
	return permissions, func() {
		v.mu.Lock()
		defer v.mu.Unlock()
		delete(v.sessions[key], ctx)
		if len(v.sessions[key]) == 0 {
			delete(v.sessions, key)
		}
	}, nil
}

// Revoke permanently invalidates a token signed by this node, including tokens
// issued before nonces were introduced. Repeated revocations are safe.
func (v *Verifier) Revoke(ctx context.Context, token string) error {
	tk, err := jwt.Parse([]byte(token), v.verifier)
	if err != nil {
		return err
	}
	key := tokenKey(tk)
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.store.Put(ctx, key, nil); err != nil {
		return fmt.Errorf("persist token revocation: %w", err)
	}
	err = v.store.Sync(ctx, key)
	// Even if syncing fails, reject the token and terminate its active sessions.
	for _, cancel := range v.sessions[key] {
		cancel()
	}
	delete(v.sessions, key)
	if err != nil {
		return fmt.Errorf("sync token revocation: %w", err)
	}
	return nil
}

func (v *Verifier) verify(ctx context.Context, tk *jwt.Token) ([]auth.Permission, error) {
	permissions, err := extractPermissions(tk)
	if err != nil {
		return nil, err
	}
	revoked, err := v.store.Has(ctx, tokenKey(tk))
	if err != nil {
		return nil, fmt.Errorf("check token revocation: %w", err)
	}
	if revoked {
		return nil, ErrRevoked
	}
	return permissions, nil
}

func tokenKey(tk *jwt.Token) datastore.Key {
	// Hash decoded signatures so alternate base64 encodings cannot bypass revocation.
	digest := sha256.Sum256(tk.Signature())
	return datastore.NewKey("/auth/revoked/" + hex.EncodeToString(digest[:]))
}
