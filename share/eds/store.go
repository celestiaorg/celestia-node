package eds

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/mount"
	"github.com/filecoin-project/dagstore/shard"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/celestiaorg/celestia-node/share"

	"github.com/celestiaorg/rsmt2d"
)

const (
	blocksPath     = "/blocks/"
	indexPath      = "/index/"
	transientsPath = "/transients/"

	defaultGCInterval = time.Hour
)

// Store maintains (via DAGStore) a top-level index enabling granular and efficient random access to
// every share and/or Merkle proof over every registered CARv1 file. The EDSStore provides a custom
// Blockstore interface implementation to achieve access. The main use-case is randomized sampling
// over the whole chain of EDS block data and getting data by namespace.
type Store struct {
	cancel context.CancelFunc

	dgstr  *dagstore.DAGStore
	bs     blockstore.Blockstore
	mounts *mount.Registry

	topIdx index.Inverted
	carIdx index.FullIndexRepo

	basepath   string
	gcInterval time.Duration
	// lastGCResult is only stored on the store for testing purposes.
	lastGCResult atomic.Pointer[dagstore.GCResult]
}

// NewStore creates a new EDS Store under the given basepath and datastore.
func NewStore(basepath string, ds datastore.Batching) (*Store, error) {
	err := setupPath(basepath)
	if err != nil {
		return nil, fmt.Errorf("failed to setup eds.Store directories: %w", err)
	}

	r := mount.NewRegistry()
	err = r.Register("fs", &mount.FSMount{FS: os.DirFS(basepath + blocksPath)})
	if err != nil {
		return nil, fmt.Errorf("failed to register FS mount on the registry: %w", err)
	}

	fsRepo, err := index.NewFSRepo(basepath + indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create index repository: %w", err)
	}

	invertedRepo := index.NewInverted(ds)
	dagStore, err := dagstore.NewDAGStore(
		dagstore.Config{
			TransientsDir: basepath + transientsPath,
			IndexRepo:     fsRepo,
			Datastore:     ds,
			MountRegistry: r,
			TopLevelIndex: invertedRepo,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create DAGStore: %w", err)
	}

	store := &Store{
		basepath: basepath,
		dgstr:    dagStore,
		topIdx:   invertedRepo,
		carIdx:   fsRepo,
		gcInterval: defaultGCInterval,
		mounts:   r,
	}

	store.bs, err = NewEDSBlockstore(store)
	if err != nil {
		return nil, fmt.Errorf("failed to create EDSBlockstore: %w", err)
	}

	return store, nil
}

func (s *Store) Start(context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	go s.gc(ctx)
	return s.dgstr.Start(ctx)
}

// Stop stops the underlying DAGStore.
func (s *Store) Stop(context.Context) error {
	defer s.cancel()
	return s.dgstr.Close()
}

// gc periodically removes all inactive or errored shards.
func (s *Store) gc(ctx context.Context) {
	ticker := time.NewTicker(s.gcInterval)
	// initialize empty gc result to avoid panic on access
	s.lastGCResult.Store(&dagstore.GCResult{
		Shards: make(map[shard.Key]error),
	})
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			res, err := s.dgstr.GC(ctx)
			if err != nil {
				log.Errorf("garbage collecting dagstore: %v", err)
				return
			}
			s.lastGCResult.Store(res)
		}

	}
}

// Put stores the given data square with DataRoot's hash as a key.
//
// The square is verified on the Exchange level, and Put only stores the square, trusting it.
// The resulting file stores all the shares and NMT Merkle Proofs of the EDS.
// Additionally, the file gets indexed s.t. store.Blockstore can access them.
func (s *Store) Put(ctx context.Context, root share.Root, square *rsmt2d.ExtendedDataSquare) error {
	key := root.String()
	f, err := os.OpenFile(s.basepath+blocksPath+key, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	err = WriteEDS(ctx, square, f)
	if err != nil {
		return fmt.Errorf("failed to write EDS to file: %w", err)
	}

	ch := make(chan dagstore.ShardResult, 1)
	err = s.dgstr.RegisterShard(ctx, shard.KeyFromString(key), &mount.FSMount{
		FS:   os.DirFS(s.basepath + blocksPath),
		Path: key,
	}, ch, dagstore.RegisterOpts{})
	if err != nil {
		return fmt.Errorf("failed to initiate shard registration: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case result := <-ch:
		if result.Error != nil {
			return fmt.Errorf("failed to register shard: %w", result.Error)
		}
		return nil
	}
}

// GetCAR takes a DataRoot and returns a buffered reader to the respective EDS serialized as a
// CARv1 file.
// The Reader strictly reads the CAR header and first quadrant (1/4) of the EDS, omitting all the
// NMT Merkle proofs. Integrity of the store data is not verified.
//
// Caller must Close returned reader after reading.
func (s *Store) GetCAR(ctx context.Context, root share.Root) (io.ReadCloser, error) {
	key := root.String()
	return s.getAccessor(ctx, shard.KeyFromString(key))
}

// Blockstore returns an IPFS Blockstore providing access to individual shares/nodes of all EDS
// registered on the Store. NOTE: The Blockstore does not store whole Celestia Blocks but IPFS
// blocks. We represent `shares` and NMT Merkle proofs as IPFS blocks and IPLD nodes so Bitswap can
// access those.
func (s *Store) Blockstore() blockstore.Blockstore {
	return s.bs
}

// CARBlockstore returns the IPFS Blockstore that provides access to the IPLD blocks stored in an
// individual CAR file.
func (s *Store) CARBlockstore(ctx context.Context, dataHash []byte) (dagstore.ReadBlockstore, error) {
	key := shard.KeyFromBytes(dataHash)
	accessor, err := s.getAccessor(ctx, key)
	if err != nil {
		return nil, err
	}

	return accessor.Blockstore()
}

func (s *Store) getAccessor(ctx context.Context, key shard.Key) (*dagstore.ShardAccessor, error) {
	ch := make(chan dagstore.ShardResult, 1)
	err := s.dgstr.AcquireShard(ctx, key, ch, dagstore.AcquireOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to initiate shard acquisition: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-ch:
		if result.Error != nil {
			return nil, fmt.Errorf("failed to acquire shard: %w", result.Error)
		}
		return result.Accessor, nil
	}
}

// Remove removes EDS from Store by the given share.Root and cleans up all the indexing.
func (s *Store) Remove(ctx context.Context, root share.Root) error {
	key := root.String()

	ch := make(chan dagstore.ShardResult, 1)
	err := s.dgstr.DestroyShard(ctx, shard.KeyFromString(key), ch, dagstore.DestroyOpts{})
	if err != nil {
		return fmt.Errorf("failed to initiate shard destruction: %w", err)
	}

	select {
	case result := <-ch:
		if result.Error != nil {
			return fmt.Errorf("failed to destroy shard: %w", result.Error)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	dropped, err := s.carIdx.DropFullIndex(shard.KeyFromString(key))
	if !dropped {
		log.Warnf("failed to drop index for %s", key)
	}
	if err != nil {
		return fmt.Errorf("failed to drop index for %s: %w", key, err)
	}

	err = os.Remove(s.basepath + blocksPath + key)
	if err != nil {
		return fmt.Errorf("failed to remove CAR file: %w", err)
	}
	return nil
}

// Get reads EDS out of Store by given DataRoot.
//
// It reads only one quadrant(1/4) of the EDS and verifies the integrity of the stored data by
// recomputing it.
func (s *Store) Get(ctx context.Context, root share.Root) (*rsmt2d.ExtendedDataSquare, error) {
	f, err := s.GetCAR(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("failed to get CAR file: %w", err)
	}
	eds, err := ReadEDS(ctx, f, root)
	if err != nil {
		return nil, fmt.Errorf("failed to read EDS from CAR file: %w", err)
	}
	return eds, nil
}

// Has checks if EDS exists by the given share.Root.
func (s *Store) Has(ctx context.Context, root share.Root) (bool, error) {
	key := root.String()
	info, err := s.dgstr.GetShardInfo(shard.KeyFromString(key))
	if err == dagstore.ErrShardUnknown {
		return false, err
	}

	return true, info.Error
}

func setupPath(basepath string) error {
	err := os.MkdirAll(basepath+blocksPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create blocks directory: %w", err)
	}
	err = os.MkdirAll(basepath+transientsPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create transients directory: %w", err)
	}
	err = os.MkdirAll(basepath+indexPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}
	return nil
}
