package eds

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/index"
	"github.com/filecoin-project/dagstore/mount"
	"github.com/filecoin-project/dagstore/shard"
	bstore "github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-datastore"
	carv1 "github.com/ipld/go-car"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/cache"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

const (
	blocksPath     = "/blocks/"
	indexPath      = "/index/"
	transientsPath = "/transients/"
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

	bs    *blockstore
	cache atomic.Pointer[cache.DoubleCache]

	carIdx      index.FullIndexRepo
	invertedIdx *simpleInvertedIndex

	basepath   string
	gcInterval time.Duration
	// lastGCResult is only stored on the store for testing purposes.
	lastGCResult atomic.Pointer[dagstore.GCResult]

	// stripedLocks is used to synchronize parallel operations
	stripedLocks  [256]sync.Mutex
	shardFailures chan dagstore.ShardResult

	metrics *metrics
}

// NewStore creates a new EDS Store under the given basepath and datastore.
func NewStore(params *Parameters, basePath string, ds datastore.Batching) (*Store, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	err := setupPath(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to setup eds.Store directories: %w", err)
	}

	r := mount.NewRegistry()
	err = r.Register("fs", &inMemoryOnceMount{})
	if err != nil {
		return nil, fmt.Errorf("failed to register memory mount on the registry: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to register FS mount on the registry: %w", err)
	}

	fsRepo, err := index.NewFSRepo(basePath + indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create index repository: %w", err)
	}

	invertedIdx, err := newSimpleInvertedIndex(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}

	failureChan := make(chan dagstore.ShardResult)
	dagStore, err := dagstore.NewDAGStore(
		dagstore.Config{
			TransientsDir: basePath + transientsPath,
			IndexRepo:     fsRepo,
			Datastore:     ds,
			MountRegistry: r,
			TopLevelIndex: invertedIdx,
			FailureCh:     failureChan,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create DAGStore: %w", err)
	}

	recentBlocksCache, err := cache.NewAccessorCache("recent", params.RecentBlocksCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create recent blocks cache: %w", err)
	}

	blockstoreCache, err := cache.NewAccessorCache("blockstore", params.BlockstoreCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockstore cache: %w", err)
	}

	store := &Store{
		basepath:      basePath,
		dgstr:         dagStore,
		carIdx:        fsRepo,
		invertedIdx:   invertedIdx,
		gcInterval:    params.GCInterval,
		mounts:        r,
		shardFailures: failureChan,
	}
	store.bs = newBlockstore(store, ds)
	store.cache.Store(cache.NewDoubleCache(recentBlocksCache, blockstoreCache))
	return store, nil
}

func (s *Store) Start(ctx context.Context) error {
	err := s.dgstr.Start(ctx)
	if err != nil {
		return err
	}
	// start Store only if DagStore succeeds
	runCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	// initialize empty gc result to avoid panic on access
	s.lastGCResult.Store(&dagstore.GCResult{
		Shards: make(map[shard.Key]error),
	})

	if s.gcInterval != 0 {
		go s.gc(runCtx)
	}

	go s.watchForFailures(runCtx)
	return nil
}

// Stop stops the underlying DAGStore.
func (s *Store) Stop(context.Context) error {
	defer s.cancel()

	if err := s.metrics.close(); err != nil {
		log.Warnw("failed to close metrics", "err", err)
	}

	if err := s.invertedIdx.close(); err != nil {
		return err
	}
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
			tnow := time.Now()
			res, err := s.dgstr.GC(ctx)
			s.metrics.observeGCtime(ctx, time.Since(tnow), err != nil)
			if err != nil {
				log.Errorf("garbage collecting dagstore: %v", err)
				return
			}
			s.lastGCResult.Store(res)
		}
	}
}

func (s *Store) watchForFailures(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case res := <-s.shardFailures:
			log.Errorw("removing shard after failure", "key", res.Key, "err", res.Error)
			s.metrics.observeShardFailure(ctx, res.Key.String())
			k := share.MustDataHashFromString(res.Key.String())
			err := s.Remove(ctx, k)
			if err != nil {
				log.Errorw("failed to remove shard after failure", "key", res.Key, "err", err)
			}
		}
	}
}

// Put stores the given data square with DataRoot's hash as a key.
//
// The square is verified on the Exchange level, and Put only stores the square, trusting it.
// The resulting file stores all the shares and NMT Merkle Proofs of the EDS.
// Additionally, the file gets indexed s.t. store.Blockstore can access them.
func (s *Store) Put(ctx context.Context, root share.DataHash, square *rsmt2d.ExtendedDataSquare) error {
	ctx, span := tracer.Start(ctx, "store/put", trace.WithAttributes(
		attribute.Int("width", int(square.Width())),
	))

	tnow := time.Now()
	err := s.put(ctx, root, square)
	result := putOK
	switch {
	case errors.Is(err, dagstore.ErrShardExists):
		result = putExists
	case err != nil:
		result = putFailed
	}
	utils.SetStatusAndEnd(span, err)
	s.metrics.observePut(ctx, time.Since(tnow), result, square.Width())
	return err
}

func (s *Store) put(ctx context.Context, root share.DataHash, square *rsmt2d.ExtendedDataSquare) (err error) {
	lk := &s.stripedLocks[root[len(root)-1]]
	lk.Lock()
	defer lk.Unlock()

	// if root already exists, short-circuit
	if has, _ := s.Has(ctx, root); has {
		return dagstore.ErrShardExists
	}

	key := root.String()
	f, err := os.OpenFile(s.basepath+blocksPath+key, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer closeAndLog("car file", f)

	// save encoded eds into buffer
	mount := &inMemoryOnceMount{
		// TODO: buffer could be pre-allocated with capacity calculated based on eds size.
		buf:       bytes.NewBuffer(nil),
		FileMount: mount.FileMount{Path: s.basepath + blocksPath + key},
	}
	err = WriteEDS(ctx, square, mount)
	if err != nil {
		return fmt.Errorf("failed to write EDS to file: %w", err)
	}

	// write whole buffered mount data in one go to optimize i/o
	if _, err = mount.WriteTo(f); err != nil {
		return fmt.Errorf("failed to write EDS to file: %w", err)
	}

	ch := make(chan dagstore.ShardResult, 1)
	err = s.dgstr.RegisterShard(ctx, shard.KeyFromString(key), mount, ch, dagstore.RegisterOpts{})
	if err != nil {
		return fmt.Errorf("failed to initiate shard registration: %w", err)
	}

	var result dagstore.ShardResult
	select {
	case result = <-ch:
	case <-ctx.Done():
		// if the context finished before the result was received, track the result in a separate goroutine
		go trackLateResult("put", ch, s.metrics, time.Minute*5)
		return ctx.Err()
	}

	if result.Error != nil {
		return fmt.Errorf("failed to register shard: %w", result.Error)
	}

	// the accessor returned in the result will be nil, so the shard needs to be acquired first to
	// become available in the cache. It might take some time, and the result should not affect the put
	// operation, so do it in a goroutine
	// TODO: Ideally, only recent blocks should be put in the cache, but there is no way right now to
	// check such a condition.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		ac, err := s.cache.Load().First().GetOrLoad(ctx, result.Key, s.getAccessor)
		if err != nil {
			log.Warnw("unable to put accessor to recent blocks accessors cache", "err", err)
			return
		}

		// need to close returned accessor to remove the reader reference
		if err := ac.Close(); err != nil {
			log.Warnw("unable to close accessor after loading", "err", err)
		}
	}()

	return nil
}

// waitForResult waits for a result from the res channel for a maximum duration specified by
// maxWait. If the result is not received within the specified duration, it logs an error
// indicating that the parent context has expired and the shard registration is stuck. If a result
// is received, it checks for any error and logs appropriate messages.
func trackLateResult(opName string, res <-chan dagstore.ShardResult, metrics *metrics, maxWait time.Duration) {
	tnow := time.Now()
	select {
	case <-time.After(maxWait):
		metrics.observeLongOp(context.Background(), opName, time.Since(tnow), longOpUnresolved)
		log.Errorf("parent context is expired, while register shard is stuck for more than %v sec", time.Since(tnow))
		return
	case result := <-res:
		// don't observe if result was received right after launch of the func
		if time.Since(tnow) < time.Second {
			return
		}
		if result.Error != nil {
			metrics.observeLongOp(context.Background(), opName, time.Since(tnow), longOpFailed)
			log.Errorf("failed to register shard after context expired: %v ago, err: %s", time.Since(tnow), result.Error)
			return
		}
		metrics.observeLongOp(context.Background(), opName, time.Since(tnow), longOpOK)
		log.Warnf("parent context expired, but register shard finished with no error,"+
			" after context expired: %v ago", time.Since(tnow))
		return
	}
}

// GetCAR takes a DataRoot and returns a buffered reader to the respective EDS serialized as a
// CARv1 file.
// The Reader strictly reads the CAR header and first quadrant (1/4) of the EDS, omitting all the
// NMT Merkle proofs. Integrity of the store data is not verified.
//
// The shard is cached in the Store, so subsequent calls to GetCAR with the same root will use the
// same reader. The cache is responsible for closing the underlying reader.
func (s *Store) GetCAR(ctx context.Context, root share.DataHash) (io.ReadCloser, error) {
	ctx, span := tracer.Start(ctx, "store/get-car")
	tnow := time.Now()
	r, err := s.getCAR(ctx, root)
	s.metrics.observeGetCAR(ctx, time.Since(tnow), err != nil)
	utils.SetStatusAndEnd(span, err)
	return r, err
}

func (s *Store) getCAR(ctx context.Context, root share.DataHash) (io.ReadCloser, error) {
	key := shard.KeyFromString(root.String())
	accessor, err := s.cache.Load().Get(key)
	if err == nil {
		return newReadCloser(accessor), nil
	}
	// If the accessor is not found in the cache, create a new one from dagstore. We don't put the
	// accessor in the cache here because getCAR is used by shrex-eds. There is a lower probability,
	// compared to other cache put triggers, that the same block will be requested again soon.
	shardAccessor, err := s.getAccessor(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get accessor: %w", err)
	}

	return newReadCloser(shardAccessor), nil
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
) (*BlockstoreCloser, error) {
	ctx, span := tracer.Start(ctx, "store/car-blockstore")
	tnow := time.Now()
	cbs, err := s.carBlockstore(ctx, root)
	s.metrics.observeCARBlockstore(ctx, time.Since(tnow), err != nil)
	utils.SetStatusAndEnd(span, err)
	return cbs, err
}

func (s *Store) carBlockstore(
	ctx context.Context,
	root share.DataHash,
) (*BlockstoreCloser, error) {
	key := shard.KeyFromString(root.String())
	accessor, err := s.cache.Load().Get(key)
	if err == nil {
		return blockstoreCloser(accessor)
	}

	// if the accessor is not found in the cache, create a new one from dagstore
	sa, err := s.getAccessor(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get accessor: %w", err)
	}
	return blockstoreCloser(sa)
}

// GetDAH returns the DataAvailabilityHeader for the EDS identified by DataHash.
func (s *Store) GetDAH(ctx context.Context, root share.DataHash) (*share.Root, error) {
	ctx, span := tracer.Start(ctx, "store/car-dah")
	tnow := time.Now()
	r, err := s.getDAH(ctx, root)
	s.metrics.observeGetDAH(ctx, time.Since(tnow), err != nil)
	utils.SetStatusAndEnd(span, err)
	return r, err
}

func (s *Store) getDAH(ctx context.Context, root share.DataHash) (*share.Root, error) {
	r, err := s.getCAR(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("eds/store: failed to get CAR file: %w", err)
	}
	defer closeAndLog("car reader", r)

	carHeader, err := carv1.ReadHeader(bufio.NewReader(r))
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
func dahFromCARHeader(carHeader *carv1.CarHeader) *share.Root {
	rootCount := len(carHeader.Roots)
	rootBytes := make([][]byte, 0, rootCount)
	for _, root := range carHeader.Roots {
		rootBytes = append(rootBytes, ipld.NamespacedSha256FromCID(root))
	}
	return &share.Root{
		RowRoots:    rootBytes[:rootCount/2],
		ColumnRoots: rootBytes[rootCount/2:],
	}
}

func (s *Store) getAccessor(ctx context.Context, key shard.Key) (cache.Accessor, error) {
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
		go trackLateResult("get_shard", ch, s.metrics, time.Minute)
		return nil, ctx.Err()
	}
}

// Remove removes EDS from Store by the given share.Root hash and cleans up all
// the indexing.
func (s *Store) Remove(ctx context.Context, root share.DataHash) error {
	ctx, span := tracer.Start(ctx, "store/remove")
	tnow := time.Now()
	err := s.remove(ctx, root)
	s.metrics.observeRemove(ctx, time.Since(tnow), err != nil)
	utils.SetStatusAndEnd(span, err)
	return err
}

func (s *Store) remove(ctx context.Context, root share.DataHash) (err error) {
	key := shard.KeyFromString(root.String())
	// remove open links to accessor from cache
	if err := s.cache.Load().Remove(key); err != nil {
		log.Warnw("remove accessor from cache", "err", err)
	}
	ch := make(chan dagstore.ShardResult, 1)
	err = s.dgstr.DestroyShard(ctx, key, ch, dagstore.DestroyOpts{})
	if err != nil {
		return fmt.Errorf("failed to initiate shard destruction: %w", err)
	}

	select {
	case result := <-ch:
		if result.Error != nil {
			return fmt.Errorf("failed to destroy shard: %w", result.Error)
		}
	case <-ctx.Done():
		go trackLateResult("remove", ch, s.metrics, time.Minute)
		return ctx.Err()
	}

	dropped, err := s.carIdx.DropFullIndex(key)
	if !dropped {
		log.Warnf("failed to drop index for %s", key)
	}
	if err != nil {
		return fmt.Errorf("failed to drop index for %s: %w", key, err)
	}

	err = os.Remove(s.basepath + blocksPath + root.String())
	if err != nil {
		return fmt.Errorf("failed to remove CAR file: %w", err)
	}
	return nil
}

// Get reads EDS out of Store by given DataRoot.
//
// It reads only one quadrant(1/4) of the EDS and verifies the integrity of the stored data by
// recomputing it.
func (s *Store) Get(ctx context.Context, root share.DataHash) (*rsmt2d.ExtendedDataSquare, error) {
	ctx, span := tracer.Start(ctx, "store/get")
	tnow := time.Now()
	eds, err := s.get(ctx, root)
	s.metrics.observeGet(ctx, time.Since(tnow), err != nil)
	utils.SetStatusAndEnd(span, err)
	return eds, err
}

func (s *Store) get(ctx context.Context, root share.DataHash) (eds *rsmt2d.ExtendedDataSquare, err error) {
	ctx, span := tracer.Start(ctx, "store/get")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()

	r, err := s.getCAR(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("failed to get CAR file: %w", err)
	}
	defer closeAndLog("car reader", r)

	eds, err = ReadEDS(ctx, r, root)
	if err != nil {
		return nil, fmt.Errorf("failed to read EDS from CAR file: %w", err)
	}
	return eds, nil
}

// Has checks if EDS exists by the given share.Root hash.
func (s *Store) Has(ctx context.Context, root share.DataHash) (has bool, err error) {
	ctx, span := tracer.Start(ctx, "store/has")
	tnow := time.Now()
	eds, err := s.has(ctx, root)
	s.metrics.observeHas(ctx, time.Since(tnow), err != nil)
	utils.SetStatusAndEnd(span, err)
	return eds, err
}

func (s *Store) has(_ context.Context, root share.DataHash) (bool, error) {
	key := root.String()
	info, err := s.dgstr.GetShardInfo(shard.KeyFromString(key))
	switch {
	case err == nil:
		return true, info.Error
	case errors.Is(err, dagstore.ErrShardUnknown):
		return false, info.Error
	default:
		return false, err
	}
}

// List lists all the registered EDSes.
func (s *Store) List() ([]share.DataHash, error) {
	ctx, span := tracer.Start(context.Background(), "store/list")
	tnow := time.Now()
	hashes, err := s.list()
	s.metrics.observeList(ctx, time.Since(tnow), err != nil)
	utils.SetStatusAndEnd(span, err)
	return hashes, err
}

func (s *Store) list() ([]share.DataHash, error) {
	shards := s.dgstr.AllShardsInfo()
	hashes := make([]share.DataHash, 0, len(shards))
	for shrd := range shards {
		hash := share.MustDataHashFromString(shrd.String())
		hashes = append(hashes, hash)
	}
	return hashes, nil
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
	buf *bytes.Buffer

	readOnce atomic.Bool
	mount.FileMount
}

func (m *inMemoryOnceMount) Fetch(ctx context.Context) (mount.Reader, error) {
	if m.buf != nil && !m.readOnce.Swap(true) {
		reader := &inMemoryReader{Reader: bytes.NewReader(m.buf.Bytes())}
		// release memory for gc, otherwise buffer will stick forever
		m.buf = nil
		return reader, nil
	}
	return m.FileMount.Fetch(ctx)
}

func (m *inMemoryOnceMount) Write(b []byte) (int, error) {
	return m.buf.Write(b)
}

func (m *inMemoryOnceMount) WriteTo(w io.Writer) (int64, error) {
	return io.Copy(w, bytes.NewReader(m.buf.Bytes()))
}

// inMemoryReader extends bytes.Reader to implement mount.Reader interface
type inMemoryReader struct {
	*bytes.Reader
}

// Close allows inMemoryReader to satisfy mount.Reader interface
func (r *inMemoryReader) Close() error {
	return nil
}
