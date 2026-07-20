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
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return nil, fmt.Errorf("revoker: read: %w", err)
	}
	var nonces []string
	if len(data) > 0 {
		if err := json.Unmarshal(data, &nonces); err != nil {
			return nil, fmt.Errorf("revoker: parse %s: %w", path, err)
		}
	}
	for _, n := range nonces {
		r.set[n] = struct{}{}
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
	tmp, err := os.CreateTemp(filepath.Dir(r.path), filepath.Base(r.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("revoker: temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("revoker: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("revoker: close: %w", err)
	}
	if err := os.Rename(tmpPath, r.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("revoker: rename: %w", err)
	}
	return nil
}
