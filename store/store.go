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

	err := ensureEmptyFile(basePath)
	if err != nil {
		return nil, fmt.Errorf("creating empty file: %w", err)
	}

	recentEDSCache, err := cache.NewAccessorCache("recent", params.RecentBlocksCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create recent eds cache: %w", err)
	}

	availabilityCache, err := cache.NewAccessorCache("availability", params.AvailabilityCacheSize)
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
) error {
	tNow := time.Now()
	lock := s.stripLock.byDatahashAndHeight(datahash, height)
	lock.lock()
	defer lock.unlock()

	path := s.basepath + blocksPath + datahash.String()
	if datahash.IsEmptyRoot() {
		err := s.ensureHeightLink(path, height)
		return err
	}

	f, err := file.CreateQ1Q4File(path, datahash, square)
	if errors.Is(err, os.ErrExist) {
		s.metrics.observePutExist(ctx)
	} else if err != nil {
		s.metrics.observePut(ctx, time.Since(tNow), square.Width(), true)
		return fmt.Errorf("creating Q1Q4 file: %w", err)
	} else {
		err = f.Close()
		if err != nil {
			s.metrics.observePut(ctx, time.Since(tNow), square.Width(), true)
			return fmt.Errorf("closing created Q1Q4 file: %w", err)
		}
	}

	// create hard link with height as name
	err = s.ensureHeightLink(path, height)
	if err != nil {
		s.metrics.observePut(ctx, time.Since(tNow), square.Width(), true)
		removeErr := s.removeFile(datahash)
		return fmt.Errorf("creating hard link: %w", errors.Join(err, removeErr))
	}
	s.metrics.observePut(ctx, time.Since(tNow), square.Width(), false)

	// put file in recent cache
	eds := &eds.Rsmt2D{ExtendedDataSquare: square}
	_, err = s.cache.First().GetOrLoad(ctx, height, accessorLoader(eds))
	if err != nil {
		log.Errorf("failed to put file in recent cache: %s", err)
	}

	return nil
}

func (s *Store) ensureHeightLink(path string, height uint64) error {
	// create hard link with height as name
	linkPath := s.basepath + heightsPath + strconv.Itoa(int(height))
	err := os.Link(path, linkPath)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("creating hard link: %w", err)
	}
	return nil
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
	path := s.basepath + blocksPath + datahash.String()
	return s.openFile(path)
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
	path := s.basepath + heightsPath + strconv.Itoa(int(height))
	return s.openFile(path)
}

func (s *Store) openFile(path string) (eds.AccessorStreamer, error) {
	f, err := file.OpenQ1Q4File(path)
	if err == nil {
		return wrapAccessor(f), nil
	}
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if errors.Is(err, file.ErrEmptyFile) {
		return emptyAccessor, nil
	}
	return nil, fmt.Errorf("opening file: %w", err)
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

	path := s.basepath + heightsPath + strconv.Itoa(int(height))
	return pathExists(path)
}

func (s *Store) Remove(ctx context.Context, height uint64, dataRoot share.DataHash) error {
	tNow := time.Now()
	err := s.remove(height, dataRoot)
	s.metrics.observeRemove(ctx, time.Since(tNow), err != nil)
	return err
}

func (s *Store) remove(height uint64, dataRoot share.DataHash) error {
	lock := s.stripLock.byHeight(height)
	lock.Lock()
	if err := s.removeLink(height); err != nil {
		return fmt.Errorf("removing link: %w", err)
	}
	lock.Unlock()

	dlock := s.stripLock.byDatahash(dataRoot)
	dlock.Lock()
	defer dlock.Unlock()
	if err := s.removeFile(dataRoot); err != nil {
		return fmt.Errorf("removing file: %w", err)
	}
	return nil
}

func (s *Store) removeLink(height uint64) error {
	if err := s.cache.Remove(height); err != nil {
		return fmt.Errorf("removing from cache: %w", err)
	}

	// remove hard link by height
	heightPath := s.basepath + heightsPath + strconv.Itoa(int(height))
	err := os.Remove(heightPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Store) removeFile(hash share.DataHash) error {
	// we don't need to remove the empty file, it should always be there
	if hash.IsEmptyRoot() {
		return nil
	}

	hashPath := s.basepath + blocksPath + hash.String()
	err := os.Remove(hashPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func accessorLoader(accessor eds.AccessorStreamer) cache.OpenAccessorFn {
	return func(context.Context) (eds.AccessorStreamer, error) {
		return wrapAccessor(accessor), nil
	}
}

func wrapAccessor(accessor eds.AccessorStreamer) eds.AccessorStreamer {
	withCache := eds.WithProofsCache(accessor)
	closedOnce := eds.WithClosedOnce(withCache)
	sanityChecked := eds.WithValidation(closedOnce)
	accessorStreamer := eds.AccessorAndStreamer(sanityChecked, closedOnce)
	return accessorStreamer
}

func ensureFolder(path string) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
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
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func ensureEmptyFile(basepath string) error {
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
