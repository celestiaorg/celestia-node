package nodebuilder

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/dgraph-io/badger/v4/options"
	"github.com/ipfs/go-datastore"
	"github.com/mitchellh/go-homedir"

	dsbadger "github.com/celestiaorg/go-ds-badger4"

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

	cfg := constraintBadgerConfig()
	ds, err := dsbadger.NewDatastore(dataPath(f.path), cfg)
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

// constraintBadgerConfig returns BadgerDB configuration optimized for low memory usage and more frequent
// compaction which prevents memory spikes.
// This is particularly important for LNs with restricted memory resources.
//
// With the following configuration, a LN uses up to 300iB of RAM during initial sync/sampling
// and up to 200MiB during normal operation. (on 4 core CPU, 8GiB RAM droplet)
//
// With the following configuration and "-tags=jemalloc", a LN uses no more than 180MiB during initial
// sync/sampling and up to 100MiB during normal operation. (same hardware spec)
// NOTE: To enable jemalloc, build celestia-node with "-tags=jemalloc" flag, which configures Badger to
// use jemalloc instead of Go's default allocator.
//
// TODO(@Wondertan): Consider alternative less constraint configuration for FN/BN
// TODO(@Wondertan): Consider dynamic memory allocation based on available RAM
func constraintBadgerConfig() *dsbadger.Options {
	opts := dsbadger.DefaultOptions // this must be copied
	// ValueLog:
	// 2mib default => 500b - makes sure headers and samples are stored in value log
	// This *tremendously* reduces the amount of memory used by the node, up to 10 times less during
	// compaction
	opts.ValueThreshold = 500
	// make sure we don't have any limits for stored headers
	opts.ValueLogMaxEntries = 100000000
	// run value log GC more often to spread the work over time
	opts.GcInterval = time.Minute * 1
	// default 0.5 => 0.125 - makes sure value log GC is more aggressive on reclaiming disk space
	opts.GcDiscardRatio = 0.125
	// Snappy saves us ~200MiB of disk space on the mainnet chain to the commit's date
	// TODO(@Wondertan): Does it worth the overhead?
	opts.Compression = options.Snappy
	opts.BlockCacheSize = 16 << 20

	// MemTables:
	// default 64mib => 16mib - decreases memory usage and makes compaction more often
	opts.MemTableSize = 16 << 20
	// default 5 => 3
	opts.NumMemtables = 3
	// default 5 => 3
	opts.NumLevelZeroTables = 3
	// default 15 => 5 - this prevents memory growth on CPU constraint systems by blocking all writers
	opts.NumLevelZeroTablesStall = 5

	// Compaction:
	// Dynamic compactor allocation
	compactors := runtime.NumCPU() / 2
	if compactors < 2 {
		compactors = 2 // can't be less than 2
	}
	if compactors > 7 { // ensure there is no more compactors than db table levels
		compactors = 7
	}
	opts.NumCompactors = compactors
	// makes sure badger is always compacted on shutdown
	opts.CompactL0OnClose = true

	return &opts
}
