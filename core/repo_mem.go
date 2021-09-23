package core

import "sync"

// NewMemRepository creates in memory implementation of Repository
func NewMemRepository() Repository {
	return &memRepository{}
}

type memRepository struct {
	cfg  *Config
	lock sync.Mutex
}

func (m *memRepository) Config() (*Config, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.cfg, nil
}

func (m *memRepository) PutConfig(cfg *Config) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.cfg = cfg
	return nil
}

// not to accidentally forget to update mem implementation when Repository will be extended.
var _ = (Repository)(&memRepository{})
