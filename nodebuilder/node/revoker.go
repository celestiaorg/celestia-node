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

// File paths below derive from the operator-supplied store config, not user
// input, so the //nolint:gosec directives on os.ReadFile / os.CreateTemp /
// os.Rename suppress gosec's taint-analysis false positives.

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

// Revoke adds nonce to the set and persists. Empty nonces are rejected — they identify nothing.
func (r *Revoker) Revoke(nonce []byte) error {
	if len(nonce) == 0 {
		return errors.New("revoker: empty nonce")
	}
	id := hex.EncodeToString(nonce)
	r.mu.Lock()
	defer r.mu.Unlock()
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

func (r *Revoker) loadLocked() error {
	data, err := os.ReadFile(r.path) //nolint:gosec,nolintlint
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

// persistLocked writes atomically via temp+rename. Caller must hold r.mu.
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

	dir := filepath.Dir(r.path)
	pattern := filepath.Base(r.path) + ".tmp-*"
	tmp, err := os.CreateTemp(dir, pattern) //nolint:gosec,nolintlint
	if err != nil {
		return fmt.Errorf("revoker: temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("revoker: write: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("revoker: sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("revoker: close: %w", err)
	}
	if err := os.Rename(tmpPath, r.path); err != nil { //nolint:gosec,nolintlint
		os.Remove(tmpPath)
		return fmt.Errorf("revoker: rename: %w", err)
	}
	return nil
}
