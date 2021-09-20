package keystore

import (
	"fmt"
	"sync"
)

// mapKeystore is a simple in-memory Keystore implementation.
type mapKeystore struct {
	keys   map[KeyName]PrivKey
	keysLk sync.Mutex
}

// NewMapKeystore constructs in-memory Keystore.
func NewMapKeystore() Keystore {
	return &mapKeystore{keys: make(map[KeyName]PrivKey)}
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
		return PrivKey{}, fmt.Errorf("keystore: key '%s' not found", n)
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
