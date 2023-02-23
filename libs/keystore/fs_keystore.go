package keystore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
)

// ErrNotFound is returned when the key does not exist.
var ErrNotFound = errors.New("keystore: key not found")

// fsKeystore implements persistent Keystore over OS filesystem.
type fsKeystore struct {
	path string

	ring keyring.Keyring
}

// NewFSKeystore creates a new Keystore over OS filesystem.
// The path must point to a directory. It is created automatically if necessary.
func NewFSKeystore(path string, ring keyring.Keyring) (Keystore, error) {
	err := os.Mkdir(path, 0755)
	if err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("keystore: failed to make a dir: %w", err)
	}
	return &fsKeystore{
		path: path,
		ring: ring,
	}, nil
}

func (f *fsKeystore) Put(n KeyName, pk PrivKey) error {
	path := f.pathTo(n.Base32())

	_, err := os.Stat(path)
	if err == nil {
		return fmt.Errorf("keystore: key '%s' already exists", n)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("keystore: check before writing key '%s' failed: %w", n, err)
	}

	data, err := json.Marshal(pk)
	if err != nil {
		return fmt.Errorf("keystore: failed to marshal key '%s': %w", n, err)
	}

	err = os.WriteFile(path, data, 0600)
	if err != nil {
		return fmt.Errorf("keystore: failed to write key '%s': %w", n, err)
	}
	return nil
}

func (f *fsKeystore) Get(n KeyName) (PrivKey, error) {
	path := f.pathTo(n.Base32())

	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PrivKey{}, fmt.Errorf("%w: %s", ErrNotFound, n)
		}

		return PrivKey{}, fmt.Errorf("keystore: check before reading key '%s' failed: %w", n, err)
	}

	if err := checkPerms(st.Mode()); err != nil {
		return PrivKey{}, fmt.Errorf("keystore: permissions of key '%s' are too relaxed: %w", n, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return PrivKey{}, fmt.Errorf("keystore: failed read key '%s': %w", n, err)
	}

	var key PrivKey
	err = json.Unmarshal(data, &key)
	if err != nil {
		return PrivKey{}, fmt.Errorf("keystore: failed to unmarshal key '%s': %w", n, err)
	}
	return key, nil
}

func (f *fsKeystore) Delete(n KeyName) error {
	path := f.pathTo(n.Base32())

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("keystore: key '%s' not found", n)
	} else if err != nil {
		return fmt.Errorf("keystore: check before reading key '%s' failed: %w", n, err)
	}

	err = os.Remove(path)
	if err != nil {
		return fmt.Errorf("keystore: failed to delete key '%s': %w", n, err)
	}
	return nil
}

func (f *fsKeystore) List() ([]KeyName, error) {
	entries, err := fs.ReadDir(os.DirFS(filepath.Dir(f.path)), filepath.Base(f.path))
	if err != nil {
		return nil, err
	}

	names := make([]KeyName, len(entries))
	for i, e := range entries {
		kn, err := KeyNameFromBase32(e.Name())
		if err != nil {
			return nil, err
		}

		if err := checkPerms(e.Type()); err != nil {
			return nil, fmt.Errorf("keystore: permissions of key '%s' are too relaxed: %w", kn, err)
		}

		names[i] = kn
	}

	return names, nil
}

func (f *fsKeystore) Path() string {
	return f.path
}

func (f *fsKeystore) Keyring() keyring.Keyring {
	return f.ring
}

func (f *fsKeystore) pathTo(file string) string {
	return filepath.Join(f.path, file)
}

func checkPerms(perms os.FileMode) error {
	if perms&0077 != 0 {
		return fmt.Errorf("required: 0600, got: %#o", perms)
	}
	return nil
}
