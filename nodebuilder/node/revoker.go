package node

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Revoker is a persistent set of revoked JWT nonces.
type Revoker struct {
	path string
	mu   sync.RWMutex
	set  map[string]struct{}
}

// NewRevoker loads (or initializes) the revocation set at path.
func NewRevoker(path string) (*Revoker, error) {
	r := &Revoker{path: path, set: map[string]struct{}{}}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("revoker: mkdir: %w", err)
	}
	if err := r.loadLocked(); err != nil {
		return nil, err
	}
	return r, nil
}

// Revoke persists the nonce; empty nonces are rejected to avoid matching every token missing the claim.
func (r *Revoker) Revoke(nonce []byte) error {
	if len(nonce) == 0 {
		return errors.New("revoker: empty nonce")
	}
	id := hex.EncodeToString(nonce)
	r.mu.Lock()
	defer r.mu.Unlock()
	// Merge any external writes (e.g. from the offline CLI) into memory before we persist,
	// otherwise the next write would silently overwrite them.
	if err := r.loadLocked(); err != nil {
		return err
	}
	if _, ok := r.set[id]; ok {
		return nil
	}
	r.set[id] = struct{}{}
	if err := r.persistLocked(); err != nil {
		delete(r.set, id)
		return err
	}
	return nil
}

// IsRevoked reports whether the given nonce has been revoked.
func (r *Revoker) IsRevoked(nonce []byte) bool {
	if len(nonce) == 0 {
		return false
	}
	id := hex.EncodeToString(nonce)
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.set[id]
	return ok
}

// List returns the revoked nonces as sorted hex strings.
func (r *Revoker) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.set))
	for id := range r.set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// loadLocked reads the on-disk set into r.set. Missing file is not an error. Caller must hold r.mu.
func (r *Revoker) loadLocked() error {
	// path is derived from the operator-supplied store config, not user input.
	data, err := os.ReadFile(r.path) //nolint:gosec,nolintlint // G304/G703 false positive
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("revoker: read: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	var nonces []string
	if err := json.Unmarshal(data, &nonces); err != nil {
		return fmt.Errorf("revoker: parse %s: %w", r.path, err)
	}
	for _, n := range nonces {
		r.set[n] = struct{}{}
	}
	return nil
}

// persistLocked writes atomically via temp+fsync+rename. Caller must hold r.mu.
func (r *Revoker) persistLocked() error {
	nonces := make([]string, 0, len(r.set))
	for id := range r.set {
		nonces = append(nonces, id)
	}
	sort.Strings(nonces)
	data, err := json.Marshal(nonces)
	if err != nil {
		return fmt.Errorf("revoker: marshal: %w", err)
	}

	// dir/base derive from the operator-supplied store config, not user input.
	dir := filepath.Dir(r.path)
	base := filepath.Base(r.path) + ".tmp-*"
	tmp, err := os.CreateTemp(dir, base) //nolint:gosec,nolintlint // G304/G703 false positive
	if err != nil {
		return fmt.Errorf("revoker: temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = os.Remove(tmpPath)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("revoker: write: %w", err)
	}
	// fsync before rename so a crash between close and OS flush cannot leave an empty
	// revoked.json — a critical failure mode for a security-sensitive denylist.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("revoker: sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("revoker: close: %w", err)
	}
	// paths derive from the operator-supplied store config, not user input.
	if err := os.Rename(tmpPath, r.path); err != nil { //nolint:gosec,nolintlint // G304/G703 false positive
		cleanup()
		return fmt.Errorf("revoker: rename: %w", err)
	}
	return nil
}
