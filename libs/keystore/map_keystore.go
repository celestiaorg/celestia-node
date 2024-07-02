package keystore

import (
	"fmt"
	"sync"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/celestiaorg/celestia-app/v2/app"
	"github.com/celestiaorg/celestia-app/v2/app/encoding"
)

// mapKeystore is a simple in-memory Keystore implementation.
type mapKeystore struct {
	keys   map[KeyName]PrivKey
	keysLk sync.Mutex
	ring   keyring.Keyring
}

// NewMapKeystore constructs in-memory Keystore.
func NewMapKeystore() Keystore {
	return &mapKeystore{
		keys: make(map[KeyName]PrivKey),
		ring: keyring.NewInMemory(encoding.MakeConfig(app.ModuleEncodingRegisters...).Codec),
	}
}

func (m *mapKeystore) Put(n KeyName, k PrivKey) error {
	m.keysLk.Lock()
	defer m.keysLk.Unlock()

	_, ok := m.keys[n]
	if ok {
		return fmt.Errorf("keystore: key '%s' already exists", n)
	}

	m.keys[n] = k
	return nil
}

func (m *mapKeystore) Get(n KeyName) (PrivKey, error) {
	m.keysLk.Lock()
	defer m.keysLk.Unlock()

	k, ok := m.keys[n]
	if !ok {
		return PrivKey{}, fmt.Errorf("%w: %s", ErrNotFound, n)
	}

	return k, nil
}

func (m *mapKeystore) Delete(n KeyName) error {
	m.keysLk.Lock()
	defer m.keysLk.Unlock()

	_, ok := m.keys[n]
	if !ok {
		return fmt.Errorf("keystore: key '%s' not found", n)
	}

	delete(m.keys, n)
	return nil
}

func (m *mapKeystore) List() ([]KeyName, error) {
	m.keysLk.Lock()
	defer m.keysLk.Unlock()

	keys := make([]KeyName, 0, len(m.keys))
	for k := range m.keys {
		keys = append(keys, k)
	}

	return keys, nil
}

func (m *mapKeystore) Path() string {
	return ""
}

func (m *mapKeystore) Keyring() keyring.Keyring {
	return m.ring
}
