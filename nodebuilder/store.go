package nodebuilder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/dgraph-io/badger/v4/options"
	"github.com/gofrs/flock"
	"github.com/ipfs/go-datastore"
	dsbadger "github.com/ipfs/go-ds-badger4"
	"github.com/mitchellh/go-homedir"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/libs/keystore"
	nodemod "github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

var (
	// ErrOpened is thrown on attempt to open already open/in-use Store.
	ErrOpened = errors.New("node: store is in use")
	// ErrNotInited is thrown on attempt to open Store without initialization.
	ErrNotInited = errors.New("node: store is not initialized")
	// ErrNoOpenStore is thrown when no opened Store is found, indicating that no node is running.
	ErrNoOpenStore = errors.New("no opened Node Store found (no node is running)")
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

	flk := flock.New(lockPath(path))
	ok, err := flk.TryLock()
	if err != nil {
		return nil, fmt.Errorf("locking file: %w", err)
	}
	if !ok {
		return nil, ErrOpened
	}

	if !IsInit(path) {
		err := errors.Join(ErrNotInited, flk.Unlock())
		return nil, err
	}

	ks, err := keystore.NewFSKeystore(keysPath(path), ring)
	if err != nil {
		err = errors.Join(err, flk.Unlock())
		return nil, err
	}

	return &fsStore{
		path:    path,
		dirLock: flk,
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
	err = errors.Join(err, f.dirLock.Close())
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
	dirLock *flock.Flock // protects directory
}

// DiscoverOpened finds a path of an opened Node Store and returns its path.
// If multiple nodes are running, it only returns the path of the first found node.
// Network is favored over node type.
//
// Network preference order: Mainnet, Mocha, Arabica, Private, Custom
// Type preference order: Bridge, Full, Light
func DiscoverOpened() (string, error) {
	defaultNetwork := p2p.GetNetworks()
	nodeTypes := nodemod.GetTypes()

	for _, n := range defaultNetwork {
		for _, tp := range nodeTypes {
			path, err := DefaultNodeStorePath(tp, n)
			if err != nil {
				return "", err
			}

			ok, _ := IsOpened(path)
			if ok {
				return path, nil
			}
		}
	}

	return "", ErrNoOpenStore
}

// DefaultNodeStorePath constructs the default node store path using the given
// node type and network.
var DefaultNodeStorePath = func(tp nodemod.Type, network p2p.Network) (string, error) {
	home := os.Getenv("CELESTIA_HOME")
	if home != "" {
		return home, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if network == p2p.Mainnet {
		return fmt.Sprintf("%s/.celestia-%s", home, strings.ToLower(tp.String())), nil
	}
	// only include network name in path for testnets and custom networks
	return fmt.Sprintf(
		"%s/.celestia-%s-%s",
		home,
		strings.ToLower(tp.String()),
		strings.ToLower(network.String()),
	), nil
}

// IsOpened checks if the Store is opened in a directory by checking its file lock.
func IsOpened(path string) (bool, error) {
	flk := flock.New(lockPath(path))
	ok, err := flk.TryLock()
	if err != nil {
		return false, fmt.Errorf("locking file: %w", err)
	}

	err = flk.Unlock()
	if err != nil {
		return false, fmt.Errorf("unlocking file: %w", err)
	}

	return !ok, nil
}

func storePath(path string) (string, error) {
	return homedir.Expand(filepath.Clean(path))
}

func configPath(base string) string {
	return filepath.Join(base, "config.toml")
}

func lockPath(base string) string {
	return filepath.Join(base, ".lock")
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
	// 2mib default => libshare.ShareSize - makes sure headers and samples are stored in value log
	// This *tremendously* reduces the amount of memory used by the node, up to 10 times less during
	// compaction
	opts.ValueThreshold = libshare.ShareSize
	// make sure we don't have any limits for stored headers
	opts.ValueLogMaxEntries = 100000000
	// run value log GC more often to spread the work over time
	opts.GcInterval = time.Minute * 1
	// default 0.5 => 0.125 - makes sure value log GC is more aggressive on reclaiming disk space
	opts.GcDiscardRatio = 0.125

	// badger stores checksum for every value, but doesn't verify it by default
	// enabling this option may allow us to see detect corrupted data
	opts.ChecksumVerificationMode = options.OnBlockRead
	opts.VerifyValueChecksum = true
	// default 64mib => 0 - disable block cache
	// most of our component maintain their own caches, so this is not needed
	opts.BlockCacheSize = 0
	// not much gain as it compresses the LSM only as well compression requires block cache
	opts.Compression = options.None

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
	compactors := max(runtime.NumCPU()/2, 2)
	if compactors > opts.MaxLevels { // ensure there is no more compactors than db table levels
		compactors = opts.MaxLevels
	}
	opts.NumCompactors = compactors
	// makes sure badger is always compacted on shutdown
	opts.CompactL0OnClose = true

	return &opts
}
