package core

import (
	"errors"

	"github.com/celestiaorg/celestia-core/config"
)

// ErrNotInited is used to signal when core is intended to be started without being initialized.
var ErrNotInited = errors.New("core: repository is not initialized")

// TODO(@Wondertan):
//  * This should expose GenesisDoc and others. We can add ad-hoc
//  * Ideally, add private keys to Keystore to unify access pattern.
type Repository interface {
	// Config loads a Config.
	Config() (*Config, error)
	// PutConfig saves given config.
	PutConfig(*Config) error
}

// Open instantiates FileSystem(FS) Repository from a given 'path'.
func Open(path string) (Repository, error) {
	if !IsInit(path) {
		return nil, ErrNotInited
	}

	return &fsRepository{path}, nil
}

func (f *fsRepository) Config() (*Config, error) {
	return LoadConfig(configPath(f.path))
}

func (f *fsRepository) PutConfig(cfg *Config) error {
	cfg.SetRoot(f.path) // not allowed to change root in runtime
	updateDefaults(cfg) // ensure stability over path fields
	config.WriteConfigFile(configPath(f.path), cfg)
	return nil
}

type fsRepository struct {
	path string
}
