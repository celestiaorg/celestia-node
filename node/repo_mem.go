package node

import (
	"sync"

	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/libs/keystore"
)

type memRepository struct {
	keys keystore.Keystore
	data datastore.Batching
	core core.Repository
	cfg  *Config
	cfgL sync.Mutex
}

// NewMemRepository creates an in-memory Repository for Node.
// Useful for testing.
func NewMemRepository(cfg *Config) Repository {
	return &memRepository{
		keys: keystore.NewMapKeystore(),
		data: datastore.NewMapDatastore(),
		core: core.NewMemRepository(),
		cfg: cfg,
	}
}

func (m *memRepository) Keystore() (keystore.Keystore, error) {
	return m.keys, nil
}

func (m *memRepository) Datastore() (datastore.Batching, error) {
	return m.data, nil
}

func (m *memRepository) Core() (core.Repository, error) {
	return m.core, nil
}

func (m *memRepository) Config() (*Config, error) {
	m.cfgL.Lock()
	defer m.cfgL.Unlock()
	return m.cfg, nil
}

func (m *memRepository) PutConfig(cfg *Config) error {
	m.cfgL.Lock()
	defer m.cfgL.Unlock()
	m.cfg = cfg
	return nil
}

func (m *memRepository) Path() string {
	return ""
}

func (m *memRepository) Close() error {
	return nil
}

