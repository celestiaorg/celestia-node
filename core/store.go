package core

import (
	"errors"

	"github.com/tendermint/tendermint/config"
)

// ErrNotInited is used to signal when core is intended to be started without being initialized.
var ErrNotInited = errors.New("core: store is not initialized")

// RepoLoader loads the Store.
type RepoLoader func() (Store, error)

// TODO(@Wondertan):
//  * This should expose GenesisDoc and others. We can add them ad-hoc.
//  * Ideally, add private keys to Keystore to unify access pattern.
type Store interface {
	// Config loads a Config.
	Config() (*Config, error)
	// PutConfig saves given config.
	PutConfig(*Config) error
}

// OpenStore instantiates FileSystem(FS) Store from a given 'path'.
func OpenStore(path string) (Store, error) {
	if !IsInit(path) {
		return nil, ErrNotInited
	}

	return &fsStore{path}, nil
}

func (f *fsStore) Config() (*Config, error) {
	return LoadConfig(configPath(f.path))
}

func (f *fsStore) PutConfig(cfg *Config) error {
	cfg.SetRoot(f.path) // not allowed changing root in runtime
	updateDefaults(cfg) // ensure stability over path fields
	config.WriteConfigFile(configPath(f.path), cfg)
	return nil
}

type fsStore struct {
	path string
}
