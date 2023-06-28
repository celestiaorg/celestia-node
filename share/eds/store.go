package eds

import (
	"bufio"
	"bytes"
	"context"
	"errors"
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
	bstore "github.com/ipfs/go-ipfs-blockstore"
	carv1 "github.com/ipld/go-car"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

const (
	blocksPath     = "/blocks/"
	indexPath      = "/index/"
	transientsPath = "/transients/"

	defaultGCInterval = time.Hour
)

var ErrNotFound = errors.New("eds not found in store")

// Store maintains (via DAGStore) a top-level index enabling granular and efficient random access to
// every share and/or Merkle proof over every registered CARv1 file. The EDSStore provides a custom
// blockstore interface implementation to achieve access. The main use-case is randomized sampling
// over the whole chain of EDS block data and getting data by namespace.
type Store struct {
	cancel context.CancelFunc

	dgstr  *dagstore.DAGStore
	mounts *mount.Registry

	cache *blockstoreCache
	bs    bstore.Blockstore

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
	err = r.Register("fs", &mount.FileMount{Path: basepath + blocksPath})
	if err != nil {
		return nil, fmt.Errorf("failed to register FS mount on the registry: %w", err)
	}

	fsRepo, err := index.NewFSRepo(basepath + indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create index repository: %w", err)
	}

	invertedRepo := newSimpleInvertedIndex(ds)
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

	cache, err := newBlockstoreCache(defaultCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockstore cache: %w", err)
	}

	store := &Store{
		basepath:   basepath,
		dgstr:      dagStore,
		topIdx:     invertedRepo,
		carIdx:     fsRepo,
		gcInterval: defaultGCInterval,
		mounts:     r,
		cache:      cache,
	}
	store.bs = newBlockstore(store, cache)
	return store, nil
}

func (s *Store) Start(ctx context.Context) error {
	err := s.dgstr.Start(ctx)
	if err != nil {
		return err
	}
	// start Store only if DagStore succeeds
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	// initialize empty gc result to avoid panic on access
	s.lastGCResult.Store(&dagstore.GCResult{
		Shards: make(map[shard.Key]error),
	})
	go s.gc(ctx)
	return nil
}

// Stop stops the underlying DAGStore.
func (s *Store) Stop(context.Context) error {
	defer s.cancel()
	return s.dgstr.Close()
}

// gc periodically removes all inactive or errored shards.
func (s *Store) gc(ctx context.Context) {
	ticker := time.NewTicker(s.gcInterval)
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
func (s *Store) Put(ctx context.Context, root share.DataHash, square *rsmt2d.ExtendedDataSquare) (err error) {
	// if root already exists, short-circuit
	has, err := s.Has(ctx, root)
	if err != nil {
		return fmt.Errorf("failed to check if root already exists in index: %w", err)
	}
	if has {
		return dagstore.ErrShardExists
	}

	ctx, span := tracer.Start(ctx, "store/put", trace.WithAttributes(
		attribute.String("root", root.String()),
		attribute.Int("width", int(square.Width())),
	))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	key := root.String()
	f, err := os.OpenFile(s.basepath+blocksPath+key, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	// save encoded eds into buffer
	buf := bytes.NewBuffer(nil)
	err = WriteEDS(ctx, square, buf)
	if err != nil {
		return fmt.Errorf("failed to write EDS to file: %w", err)
	}

	// write whole buffer in one go to optimise i/o
	if _, err = f.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write EDS to file: %w", err)
	}

	mount := &inMemoryOnceMount{
		buf:   buf.Bytes(),
		Mount: &mount.FileMount{Path: s.basepath + blocksPath + key},
	}
	ch := make(chan dagstore.ShardResult, 1)
	err = s.dgstr.RegisterShard(ctx, shard.KeyFromString(key), mount, ch, dagstore.RegisterOpts{})
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
// The shard is cached in the Store, so subsequent calls to GetCAR with the same root will use the
// same reader. The cache is responsible for closing the underlying reader.
func (s *Store) GetCAR(ctx context.Context, root share.DataHash) (io.Reader, error) {
	ctx, span := tracer.Start(ctx, "store/get-car", trace.WithAttributes(attribute.String("root", root.String())))
	defer span.End()

	key := root.String()
	accessor, err := s.getCachedAccessor(ctx, shard.KeyFromString(key))
	if err != nil {
		return nil, fmt.Errorf("failed to get accessor: %w", err)
	}
	return accessor.sa.Reader(), nil
}

// Blockstore returns an IPFS blockstore providing access to individual shares/nodes of all EDS
// registered on the Store. NOTE: The blockstore does not store whole Celestia Blocks but IPFS
// blocks. We represent `shares` and NMT Merkle proofs as IPFS blocks and IPLD nodes so Bitswap can
// access those.
func (s *Store) Blockstore() bstore.Blockstore {
	return s.bs
}

// CARBlockstore returns an IPFS Blockstore providing access to individual shares/nodes of a
// specific EDS identified by DataHash and registered on the Store. NOTE: The Blockstore does not
// store whole Celestia Blocks but IPFS blocks. We represent `shares` and NMT Merkle proofs as IPFS
// blocks and IPLD nodes so Bitswap can access those.
func (s *Store) CARBlockstore(
	ctx context.Context,
	root share.DataHash,
) (dagstore.ReadBlockstore, error) {
	key := shard.KeyFromString(root.String())
	accessor, err := s.getCachedAccessor(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("eds/store: failed to get accessor: %w", err)
	}
	return accessor.bs, nil
}

// GetDAH returns the DataAvailabilityHeader for the EDS identified by DataHash.
func (s *Store) GetDAH(ctx context.Context, root share.DataHash) (*share.Root, error) {
	ctx, span := tracer.Start(ctx, "store/get-dah", trace.WithAttributes(attribute.String("root", root.String())))
	defer span.End()

	key := shard.KeyFromString(root.String())
	accessor, err := s.getCachedAccessor(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("eds/store: failed to get accessor: %w", err)
	}

	carHeader, err := carv1.ReadHeader(bufio.NewReader(accessor.sa.Reader()))
	if err != nil {
		return nil, fmt.Errorf("eds/store: failed to read car header: %w", err)
	}

	dah := dahFromCARHeader(carHeader)
	if !bytes.Equal(dah.Hash(), root) {
		return nil, fmt.Errorf("eds/store: content integrity mismatch from CAR for root %x", root)
	}
	return dah, nil
}

// dahFromCARHeader returns the DataAvailabilityHeader stored in the CIDs of a CARv1 header.
func dahFromCARHeader(carHeader *carv1.CarHeader) *header.DataAvailabilityHeader {
	rootCount := len(carHeader.Roots)
	rootBytes := make([][]byte, 0, rootCount)
	for _, root := range carHeader.Roots {
		rootBytes = append(rootBytes, ipld.NamespacedSha256FromCID(root))
	}
	return &header.DataAvailabilityHeader{
		RowRoots:    rootBytes[:rootCount/2],
		ColumnRoots: rootBytes[rootCount/2:],
	}
}

func (s *Store) getAccessor(ctx context.Context, key shard.Key) (*dagstore.ShardAccessor, error) {
	ch := make(chan dagstore.ShardResult, 1)
	err := s.dgstr.AcquireShard(ctx, key, ch, dagstore.AcquireOpts{})
	if err != nil {
		if errors.Is(err, dagstore.ErrShardUnknown) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to initialize shard acquisition: %w", err)
	}

	select {
	case res := <-ch:
		if res.Error != nil {
			return nil, fmt.Errorf("failed to acquire shard: %w", res.Error)
		}
		return res.Accessor, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Store) getCachedAccessor(ctx context.Context, key shard.Key) (*accessorWithBlockstore, error) {
	lk := &s.cache.stripedLocks[shardKeyToStriped(key)]
	lk.Lock()
	defer lk.Unlock()

	accessor, err := s.cache.unsafeGet(key)
	if err != nil && err != errCacheMiss {
		log.Errorf("unexpected error while reading key from bs cache %s: %s", key, err)
	}
	if accessor != nil {
		return accessor, nil
	}

	// wasn't found in cache, so acquire it and add to cache
	shardAccessor, err := s.getAccessor(ctx, key)
	if err != nil {
		return nil, err
	}
	return s.cache.unsafeAdd(key, shardAccessor)
}

// Remove removes EDS from Store by the given share.Root hash and cleans up all
// the indexing.
func (s *Store) Remove(ctx context.Context, root share.DataHash) (err error) {
	ctx, span := tracer.Start(ctx, "store/remove", trace.WithAttributes(attribute.String("root", root.String())))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	key := root.String()
	ch := make(chan dagstore.ShardResult, 1)
	err = s.dgstr.DestroyShard(ctx, shard.KeyFromString(key), ch, dagstore.DestroyOpts{})
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
func (s *Store) Get(ctx context.Context, root share.DataHash) (eds *rsmt2d.ExtendedDataSquare, err error) {
	ctx, span := tracer.Start(ctx, "store/get", trace.WithAttributes(attribute.String("root", root.String())))
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	f, err := s.GetCAR(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("failed to get CAR file: %w", err)
	}
	eds, err = ReadEDS(ctx, f, root)
	if err != nil {
		return nil, fmt.Errorf("failed to read EDS from CAR file: %w", err)
	}
	return eds, nil
}

// Has checks if EDS exists by the given share.Root hash.
func (s *Store) Has(ctx context.Context, root share.DataHash) (bool, error) {
	_, span := tracer.Start(ctx, "store/has", trace.WithAttributes(attribute.String("root", root.String())))
	defer span.End()

	key := root.String()
	info, err := s.dgstr.GetShardInfo(shard.KeyFromString(key))
	switch err {
	case nil:
		return true, info.Error
	case dagstore.ErrShardUnknown:
		return false, info.Error
	default:
		return false, err
	}
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

// inMemoryOnceMount is used to allow reading once from buffer before using main mount.Reader
type inMemoryOnceMount struct {
	buf []byte

	readOnce atomic.Bool
	mount.Mount
}

// inmemoryReader extends bytes.Reader to implement mount.Reader interface
type inmemoryReader struct {
	*bytes.Reader
}

func (i *inmemoryReader) Close() error {
	return nil
}

func (i *inMemoryOnceMount) Fetch(ctx context.Context) (mount.Reader, error) {
	if !i.readOnce.Swap(true) {
		reader := &inmemoryReader{Reader: bytes.NewReader(i.buf)}
		// release memory for gc, otherwise buffer will stick forever
		i.buf = nil
		return reader, nil
	}
	return i.Mount.Fetch(ctx)
}
