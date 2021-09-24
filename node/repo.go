package node

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/ipfs/go-datastore"
	dsbadger "github.com/ipfs/go-ds-badger2"
	"github.com/mitchellh/go-homedir"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/libs/fslock"
	"github.com/celestiaorg/celestia-node/libs/keystore"
)

var (
	ErrOpened   = errors.New("node: repository is in use")
	ErrNotInited = errors.New("node: repository is not set up")
)

type Repository interface {
	Keystore() (keystore.Keystore, error)
	Datastore() (datastore.Batching, error)
	Core() (core.Repository, error)

	Config() (*Config, error)
	PutConfig(*Config) error

	Path() string
	Close() error

	DiskUsage() (uint64, error)
}

// TODO: Nice error wrappings
// TODO: Repo for light node
// TODO: Memory repo
type fsRepository struct {
	path string

	data    datastore.Batching
	keys    keystore.Keystore
	core    core.Repository

	lock sync.RWMutex // protects all the fields
	dirLock *fslock.Locker // protects directory
}

// Open creates new FS Repository under the given 'path'.
// The Repository must be initialized first.
func Open(path string) (Repository, error) {
	path, err := repoPath(path)
	if err != nil {
		return nil, err
	}

	flock, err := fslock.Lock(lockPath(path))
	if err != nil {
		if err == fslock.ErrLocked {
			return nil, ErrOpened
		}
		return nil, err
	}

	ok, err := IsInit(path)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotInited
	}

	return &fsRepository{
		path:    path,
		dirLock: flock,
	}, nil
}

func (f *fsRepository) Path() string {
	return f.path
}

func (f *fsRepository) Config() (*Config, error) {
	return readConfig(f.path)
}

func (f *fsRepository) PutConfig(config *Config) error {
	return writeConfig(f.path, config, true)
}

func (f *fsRepository) Keystore() (_ keystore.Keystore, err error) {
	f.lock.RLock()
	if f.keys != nil {
		f.lock.RUnlock()
		return f.keys, nil
	}
	f.lock.RUnlock()

	f.lock.Lock()
	defer f.lock.Unlock()

	f.keys, err = keystore.NewFSKeystore(keysPath(f.path))
	if err != nil {
		return nil, err
	}

	return f.keys, nil
}

func (f *fsRepository) Datastore() (_ datastore.Batching, err error) {
	f.lock.RLock()
	if f.data != nil {
		f.lock.RUnlock()
		return f.data, nil
	}
	f.lock.RUnlock()

	f.lock.Lock()
	defer f.lock.Unlock()

	// TODO(@Wondertan): Study badger code and review available options to fine tune it for our use-cases.
	opts := dsbadger.DefaultOptions // this should be copied
	f.data, err = dsbadger.NewDatastore(dataPath(f.path), &opts)
	if err != nil {
		return nil, err
	}

	return f.data, nil
}

func (f *fsRepository) Core() (_ core.Repository, err error) {
	f.lock.RLock()
	if f.core != nil {
		f.lock.RUnlock()
		return f.core, nil
	}
	f.lock.RUnlock()

	f.lock.Lock()
	defer f.lock.Unlock()

	f.core, err = core.Open(corePath(f.path))
	if err != nil {
		return nil, err
	}

	return f.core, nil
}

func (f *fsRepository) Close() error {
	defer f.dirLock.Unlock()
	return f.data.Close()
}

func (f *fsRepository) DiskUsage() (uint64, error) {
	panic("implement me")
}

func writeConfig(path string, cfg *Config, force bool) error {
	exist, err := isSetUp(configPath(path))
	if err != nil {
		return err
	}
	if exist && !force {
		return nil
	}

	f, err := os.Create(configPath(path))
	if err != nil {
		return err
	}
	defer f.Close()

	return config.WriteTo(cfg, f)
}

func readConfig(path string) (*Config, error) {
	f, err := os.Open(configPath(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return config.ReadFrom(f)
}


func setUpRepo(path string) error {
	err := os.MkdirAll(path, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}
	// check permissions to write into it
	f, err := os.Create(filepath.Join(path, ".check"))
	if err != nil {
		return err
	}
	defer f.Close()

	return os.Remove(f.Name())
}

func setUpSubDir(path string) error {
	err := os.Mkdir(path, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func isSetUp(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}

	return true, nil
}

func repoPath(path string) (string, error) {
	return homedir.Expand(filepath.Clean(path))
}

func configPath(base string) string {
	return filepath.Join(base, "config.toml")
}

func lockPath(base string) string {
	return filepath.Join(base, "lock")
}

func keysPath(base string) string {
	return filepath.Join(base, "keys")
}

func dataPath(base string) string {
	return filepath.Join(base, "data")
}

func corePath(base string) string {
	return filepath.Join(base, "core")
}
