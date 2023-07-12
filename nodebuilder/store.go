package nodebuilder

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/ipfs/go-datastore"
	dsbadger "github.com/ipfs/go-ds-badger2"
	"github.com/mitchellh/go-homedir"

	"github.com/celestiaorg/celestia-node/libs/fslock"
	"github.com/celestiaorg/celestia-node/libs/keystore"
)

var (
	// ErrOpened is thrown on attempt to open already open/in-use Store.
	ErrOpened = errors.New("node: store is in use")
	// ErrNotInited is thrown on attempt to open Store without initialization.
	ErrNotInited = errors.New("node: store is not initialized")
)

// Store encapsulates storage for the Node. Basically, it is the Store of all Stores.
// It provides access for the Node data stored in root directory e.g. '~/.celestia'.
type Store interface {
	// Path reports the FileSystem path of Store.
	Path() string

	// Keystore provides a Keystore to access keys.
	Keystore() (keystore.Keystore, error)

	// Datastore provides a Datastore - a KV store for arbitrary data to be stored on disk.
	Datastore() (datastore.Batching, error)

	// Config loads the stored Node config.
	Config() (*Config, error)

	// PutConfig alters the stored Node config.
	PutConfig(*Config) error

	// Close closes the Store freeing up acquired resources and locks.
	Close() error
}

// OpenStore creates new FS Store under the given 'path'.
// To be opened the Store must be initialized first, otherwise ErrNotInited is thrown.
// OpenStore takes a file Lock on directory, hence only one Store can be opened at a time under the
// given 'path', otherwise ErrOpened is thrown.
func OpenStore(path string, ring keyring.Keyring) (Store, error) {
	path, err := storePath(path)
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

	ok := IsInit(path)
	if !ok {
		flock.Unlock() //nolint: errcheck
		return nil, ErrNotInited
	}

	ks, err := keystore.NewFSKeystore(keysPath(path), ring)
	if err != nil {
		return nil, err
	}

	return &fsStore{
		path:    path,
		dirLock: flock,
		keys:    ks,
	}, nil
}

func (f *fsStore) Path() string {
	return f.path
}

func (f *fsStore) Config() (*Config, error) {
	cfg, err := LoadConfig(configPath(f.path))
	if err != nil {
		return nil, fmt.Errorf("node: can't load Config: %w", err)
	}

	return cfg, nil
}

func (f *fsStore) PutConfig(cfg *Config) error {
	err := SaveConfig(configPath(f.path), cfg)
	if err != nil {
		return fmt.Errorf("node: can't save Config: %w", err)
	}

	return nil
}

func (f *fsStore) Keystore() (_ keystore.Keystore, err error) {
	if f.keys == nil {
		return nil, fmt.Errorf("node: no Keystore found")
	}
	return f.keys, nil
}

func (f *fsStore) Datastore() (datastore.Batching, error) {
	f.dataMu.Lock()
	defer f.dataMu.Unlock()
	if f.data != nil {
		return f.data, nil
	}

	opts := dsbadger.DefaultOptions // this should be copied

	// Badger sets ValueThreshold to 1K by default and this makes shares being stored in LSM tree
	// instead of the value log, so we change the value to be lower than share size,
	// so shares are store in value log. For value log and LSM definitions
	opts.ValueThreshold = 128
	// We always write unique values to Badger transaction so there is no need to detect conflicts.
	opts.DetectConflicts = false
	// Use MemoryMap for better performance
	opts.ValueLogLoadingMode = options.MemoryMap
	opts.TableLoadingMode = options.MemoryMap
	// Truncate set to true will truncate corrupted data on start if there is any.
	// If we don't truncate, the node will refuse to start and will beg for recovering, etc.
	// If we truncate, the node will start with any uncorrupted data and reliably sync again what was
	// corrupted in most cases.
	opts.Truncate = true
	// MaxTableSize defines in memory and on disk size of LSM tree
	// Bigger values constantly takes more RAM
	// TODO(@Wondertan): Make configurable with more conservative defaults for Light Node
	opts.MaxTableSize = 64 << 20

	ds, err := dsbadger.NewDatastore(dataPath(f.path), &opts)
	if err != nil {
		return nil, fmt.Errorf("node: can't open Badger Datastore: %w", err)
	}

	f.data = ds
	return ds, nil
}

func (f *fsStore) Close() (err error) {
	err = errors.Join(err, f.dirLock.Unlock())
	f.dataMu.Lock()
	if f.data != nil {
		err = errors.Join(err, f.data.Close())
	}
	f.dataMu.Unlock()
	return
}

type fsStore struct {
	path string

	dataMu  sync.Mutex
	data    datastore.Batching
	keys    keystore.Keystore
	dirLock *fslock.Locker // protects directory
}

func storePath(path string) (string, error) {
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

func blocksPath(base string) string {
	return filepath.Join(base, "blocks")
}

func transientsPath(base string) string {
	// we don't actually use the transients directory anymore, but it could be populated from previous
	// versions.
	return filepath.Join(base, "transients")
}

func indexPath(base string) string {
	return filepath.Join(base, "index")
}

func dataPath(base string) string {
	return filepath.Join(base, "data")
}
