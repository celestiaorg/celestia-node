package node

import (
	"sync"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"

	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/node/node"
)

type memStore struct {
	keys keystore.Keystore
	data datastore.Batching
	cfg  *node.Config
	cfgL sync.Mutex
}

// NewMemStore creates an in-memory Store for Node.
// Useful for testing.
func NewMemStore() Store {
	return &memStore{
		keys: keystore.NewMapKeystore(),
		data: ds_sync.MutexWrap(datastore.NewMapDatastore()),
	}
}

func (m *memStore) Keystore() (keystore.Keystore, error) {
	return m.keys, nil
}

func (m *memStore) Datastore() (datastore.Batching, error) {
	return m.data, nil
}

func (m *memStore) Config() (*node.Config, error) {
	m.cfgL.Lock()
	defer m.cfgL.Unlock()
	return m.cfg, nil
}

func (m *memStore) PutConfig(cfg *node.Config) error {
	m.cfgL.Lock()
	defer m.cfgL.Unlock()
	m.cfg = cfg
	return nil
}

func (m *memStore) Path() string {
	return ""
}

func (m *memStore) Close() error {
	return nil
}
