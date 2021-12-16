package core

import "sync"

// NewMemStore creates in memory implementation of Store
func NewMemStore() Store {
	return &memStore{}
}

type memStore struct {
	cfg  *Config
	lock sync.Mutex
}

func (m *memStore) Config() (*Config, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.cfg, nil
}

func (m *memStore) PutConfig(cfg *Config) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.cfg = cfg
	return nil
}

// not to accidentally forget to update mem implementation when Store will be extended.
var _ = (Store)(&memStore{})
