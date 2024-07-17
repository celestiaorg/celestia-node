package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/store/cache"
	"github.com/celestiaorg/celestia-node/store/file"
)

var (
	log = logging.Logger("share/eds")

	emptyAccessor = &eds.Rsmt2D{ExtendedDataSquare: share.EmptyExtendedDataSquare()}
)

// TODO(@walldiss):
//  - persist store stats like
// 		- amount of files
//		- file types hist (ods/q1q4)
//		- file size hist
//		- amount of links hist
//  - add handling of corrupted files / links
//  - maintain in-memory missing files index / bloom-filter to fast return for not stored files.
//  - lock store folder
//  - add traces

const (
	blocksPath  = "/blocks/"
	heightsPath = blocksPath + "heights/"

	defaultDirPerm = 0o755
)

var ErrNotFound = errors.New("eds not found in store")

// Store is a storage for EDS files. It persists EDS files on disk in form of Q1Q4 files or ODS
// files. It provides methods to put, get and remove EDS files. It has two caches: recent eds cache
// and availability cache. Recent eds cache is used to cache recent blocks. Availability cache is
// used to cache blocks that are accessed by sample requests. Store is thread-safe.
type Store struct {
	// basepath is the root directory of the store
	basepath string
	// cache is used to cache recent blocks and blocks that are accessed frequently
	cache *cache.DoubleCache
	// stripedLocks is used to synchronize parallel operations
	stripLock *striplock
	metrics   *metrics
}

// NewStore creates a new EDS Store under the given basepath and datastore.
func NewStore(params *Parameters, basePath string) (*Store, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	// Ensure the blocks folder exists or is created.
	blocksFolderPath := basePath + blocksPath
	if err := ensureFolder(blocksFolderPath); err != nil {
		log.Errorf("Failed to ensure the existence of the blocks folder at '%s': %s", blocksFolderPath, err)
		return nil, fmt.Errorf("ensure blocks folder '%s': %w", blocksFolderPath, err)
	}

	// Ensure the heights folder exists or is created.
	heightsFolderPath := basePath + heightsPath
	if err := ensureFolder(heightsFolderPath); err != nil {
		log.Errorf("Failed to ensure the existence of the heights folder at '%s': %s", heightsFolderPath, err)
		return nil, fmt.Errorf("ensure heights folder '%s': %w", heightsFolderPath, err)
	}

	err := createEmptyFile(basePath)
	if err != nil {
		return nil, fmt.Errorf("creating empty file: %w", err)
	}

	recentEDSCache, err := cache.NewAccessorCache("recent", params.RecentBlocksCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create recent eds cache: %w", err)
	}

	availabilityCache, err := cache.NewAccessorCache("blockstore", params.AvailabilityCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create availability cache: %w", err)
	}

	store := &Store{
		basepath:  basePath,
		cache:     cache.NewDoubleCache(recentEDSCache, availabilityCache),
		stripLock: newStripLock(1024),
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.metrics.close()
}

func (s *Store) Put(
	ctx context.Context,
	datahash share.DataHash,
	height uint64,
	square *rsmt2d.ExtendedDataSquare,
) (eds.AccessorStreamer, error) {
	tNow := time.Now()
	lock := s.stripLock.byDatahashAndHeight(datahash, height)
	lock.lock()
	defer lock.unlock()

	if datahash.IsEmptyRoot() {
		err := s.addEmptyHeight(height)
		return emptyAccessor, err
	}

	// short circuit if file exists
	if has, _ := s.hasByHeight(height); has {
		s.metrics.observePutExist(ctx)
		return s.getByHeight(height)
	}

	filePath := s.basepath + blocksPath + datahash.String()
	f, err := s.createFile(filePath, datahash, square)
	if err != nil {
		s.metrics.observePut(ctx, time.Since(tNow), square.Width(), true)
		return nil, fmt.Errorf("creating file: %w", err)
	}

	// create hard link with height as name
	err = s.createHeightLink(datahash, height)
	if err != nil {
		s.metrics.observePut(ctx, time.Since(tNow), square.Width(), false)
		return nil, fmt.Errorf("linking height: %w", err)
	}
	s.metrics.observePut(ctx, time.Since(tNow), square.Width(), false)

	// put file in recent cache
	f, err = s.cache.First().GetOrLoad(ctx, height, fileLoader(f))
	if err != nil {
		log.Warnf("failed to put file in recent cache: %s", err)
	}
	return f, nil
}

func (s *Store) createFile(
	filePath string,
	datahash share.DataHash,
	square *rsmt2d.ExtendedDataSquare,
) (eds.AccessorStreamer, error) {
	// check if file with the same hash already exists
	f, err := s.getByHash(datahash)
	if err == nil {
		return f, nil
	}

	if !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("getting by hash: %w", err)
	}

	// create Q1Q4 file
	f, err = file.CreateQ1Q4File(filePath, datahash, square)
	if err != nil {
		return nil, fmt.Errorf("creating ODS file: %w", err)
	}
	return f, nil
}

func (s *Store) GetByHash(ctx context.Context, datahash share.DataHash) (eds.AccessorStreamer, error) {
	if datahash.IsEmptyRoot() {
		return emptyAccessor, nil
	}
	lock := s.stripLock.byDatahash(datahash)
	lock.RLock()
	defer lock.RUnlock()

	tNow := time.Now()
	f, err := s.getByHash(datahash)
	s.metrics.observeGet(ctx, time.Since(tNow), err != nil)
	return f, err
}

func (s *Store) getByHash(datahash share.DataHash) (eds.AccessorStreamer, error) {
	if datahash.IsEmptyRoot() {
		return emptyAccessor, nil
	}
	path := s.basepath + blocksPath + datahash.String()
	return s.openFile(path)
}

func (s *Store) createHeightLink(datahash share.DataHash, height uint64) error {
	filePath := s.basepath + blocksPath + datahash.String()
	// create hard link with height as name
	linkPath := s.basepath + heightsPath + strconv.Itoa(int(height))
	err := os.Link(filePath, linkPath)
	if err != nil {
		return fmt.Errorf("creating hard link: %w", err)
	}
	return nil
}

func (s *Store) GetByHeight(ctx context.Context, height uint64) (eds.AccessorStreamer, error) {
	lock := s.stripLock.byHeight(height)
	lock.RLock()
	defer lock.RUnlock()

	tNow := time.Now()
	f, err := s.getByHeight(height)
	s.metrics.observeGet(ctx, time.Since(tNow), err != nil)
	return f, err
}

func (s *Store) getByHeight(height uint64) (eds.AccessorStreamer, error) {
	f, err := s.cache.Get(height)
	if err == nil {
		return f, nil
	}
	path := s.basepath + heightsPath + fmt.Sprintf("%d", height)
	return s.openFile(path)
}

func (s *Store) HasByHash(ctx context.Context, datahash share.DataHash) (bool, error) {
	if datahash.IsEmptyRoot() {
		return true, nil
	}
	lock := s.stripLock.byDatahash(datahash)
	lock.RLock()
	defer lock.RUnlock()

	tNow := time.Now()
	exist, err := s.hasByHash(datahash)
	s.metrics.observeHas(ctx, time.Since(tNow), err != nil)
	return exist, err
}

func (s *Store) hasByHash(datahash share.DataHash) (bool, error) {
	if datahash.IsEmptyRoot() {
		return true, nil
	}
	path := s.basepath + blocksPath + datahash.String()
	return pathExists(path)
}

func (s *Store) HasByHeight(ctx context.Context, height uint64) (bool, error) {
	lock := s.stripLock.byHeight(height)
	lock.RLock()
	defer lock.RUnlock()

	tNow := time.Now()
	exist, err := s.hasByHeight(height)
	s.metrics.observeHas(ctx, time.Since(tNow), err != nil)
	return exist, err
}

func (s *Store) hasByHeight(height uint64) (bool, error) {
	_, err := s.cache.Get(height)
	if err == nil {
		return true, nil
	}

	path := s.basepath + heightsPath + fmt.Sprintf("%d", height)
	return pathExists(path)
}

func (s *Store) Remove(ctx context.Context, height uint64) error {
	lock := s.stripLock.byHeight(height)
	lock.Lock()
	defer lock.Unlock()

	tNow := time.Now()
	err := s.remove(ctx, height)
	s.metrics.observeRemove(ctx, time.Since(tNow), err != nil)
	return err
}

func (s *Store) remove(ctx context.Context, height uint64) error {
	f, err := s.getByHeight(height)
	if err != nil {
		// short circuit if file not exists
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return fmt.Errorf("getting by height: %w", err)
	}

	hash, err := f.DataRoot(ctx)
	if err != nil {
		return fmt.Errorf("getting data hash: %w", err)
	}
	// close file to release the reference in the cache
	if err = f.Close(); err != nil {
		return fmt.Errorf("closing file on removal: %w", err)
	}

	// lock by datahash to prevent concurrent access to the same underlying file
	// by GetByHash
	dlock := s.stripLock.byDatahash(hash)
	dlock.Lock()
	defer dlock.Unlock()

	if err = s.cache.Remove(height); err != nil {
		return fmt.Errorf("removing from cache: %w", err)
	}

	// remove hard link by height
	heightPath := s.basepath + heightsPath + fmt.Sprintf("%d", height)
	if err = os.Remove(heightPath); err != nil {
		return fmt.Errorf("removing by height: %w", err)
	}

	// remove file if not empty root
	if !hash.IsEmptyRoot() {
		hashPath := s.basepath + blocksPath + hash.String()
		err = os.Remove(hashPath)
		if err != nil {
			return fmt.Errorf("removing by hash: %w", err)
		}
	}
	return nil
}

func (s *Store) openFile(path string) (eds.AccessorStreamer, error) {
	f, err := file.OpenQ1Q4File(path)
	if err == nil {
		return wrappedFile(f), nil
	}
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if errors.Is(err, file.ErrFileIsEmpty) {
		return emptyAccessor, nil
	}
	return nil, fmt.Errorf("opening file: %w", err)
}

func fileLoader(f eds.AccessorStreamer) cache.OpenAccessorFn {
	return func(context.Context) (eds.AccessorStreamer, error) {
		return wrappedFile(f), nil
	}
}

func wrappedFile(f eds.AccessorStreamer) eds.AccessorStreamer {
	withCache := eds.WithProofsCache(f)
	closedOnce := eds.WithClosedOnce(withCache)
	sanityChecked := eds.WithValidation(closedOnce)
	accessorStreamer := eds.AccessorAndStreamer(sanityChecked, closedOnce)
	return accessorStreamer
}

func ensureFolder(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		err = os.Mkdir(path, defaultDirPerm)
		if err != nil {
			return fmt.Errorf("creating blocks dir: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking dir: %w", err)
	}
	if !info.IsDir() {
		return errors.New("expected dir, got a file")
	}
	return nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *Store) addEmptyHeight(height uint64) error {
	// short circuit if link exists
	has, err := s.hasByHeight(height)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	return s.createHeightLink(share.EmptyRoot().Hash(), height)
}

func createEmptyFile(basepath string) error {
	path := basepath + blocksPath + share.DataHash(share.EmptyRoot().Hash()).String()
	ok, err := pathExists(path)
	if err != nil {
		return fmt.Errorf("checking empty root: %w", err)
	}
	if ok {
		return nil
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating empty root file: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("closing empty root file: %w", err)
	}
	return nil
}
