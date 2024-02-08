package store

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/store/cache"
	"github.com/celestiaorg/celestia-node/share/store/file"
)

var (
	log    = logging.Logger("share/eds")
	tracer = otel.Tracer("share/eds")

	emptyFile = &file.MemFile{Eds: share.EmptyExtendedDataSquare()}
)

// TODO(@walldiss):
//  - index empty files by height
//  - persist store stats like amount of files, file types, avg file size etc in a file
//  - handle corrupted files
//  - maintain in-memory missing files index / bloom-filter to fast return for not stored files.
//  - lock store folder

const (
	hashsPath    = "/blocks/"
	heightsPath  = "/heights/"
	emptyHeights = "/empty_heights"

	defaultDirPerm = 0755
)

var ErrNotFound = errors.New("eds not found in store")

// Store maintains (via DAGStore) a top-level index enabling granular and efficient random access to
// every share and/or Merkle proof over every registered CARv1 file. The EDSStore provides a custom
// blockstore interface implementation to achieve access. The main use-case is randomized sampling
// over the whole chain of EDS block data and getting data by namespace.
type Store struct {
	// basepath is the root directory of the store
	basepath string
	// cache is used to cache recent blocks and blocks that are accessed frequently
	cache *cache.DoubleCache
	// stripedLocks is used to synchronize parallel operations
	stripLock *striplock
	// emptyHeights stores the heights of empty files
	emptyHeights     map[uint64]struct{}
	emptyHeightsLock sync.RWMutex

	metrics *metrics
}

// NewStore creates a new EDS Store under the given basepath and datastore.
func NewStore(params *Parameters, basePath string) (*Store, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	// ensure blocks folder
	if err := ensureFolder(basePath + hashsPath); err != nil {
		return nil, fmt.Errorf("ensure blocks folder: %w", err)
	}

	// ensure heights folder
	if err := ensureFolder(basePath + heightsPath); err != nil {
		return nil, fmt.Errorf("ensure blocks folder: %w", err)
	}

	// ensure empty heights file
	if err := ensureFile(basePath + emptyHeights); err != nil {
		return nil, fmt.Errorf("ensure empty heights file: %w", err)
	}

	recentBlocksCache, err := cache.NewFileCache("recent", params.RecentBlocksCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create recent blocks cache: %w", err)
	}

	blockstoreCache, err := cache.NewFileCache("blockstore", params.BlockstoreCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockstore cache: %w", err)
	}

	emptyHeights, err := loadEmptyHeights(basePath)
	if err != nil {
		return nil, fmt.Errorf("loading empty heights: %w", err)
	}

	store := &Store{
		basepath:     basePath,
		cache:        cache.NewDoubleCache(recentBlocksCache, blockstoreCache),
		stripLock:    newStripLock(1024),
		emptyHeights: emptyHeights,
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.storeEmptyHeights()
}

func (s *Store) Put(
	ctx context.Context,
	datahash share.DataHash,
	height uint64,
	square *rsmt2d.ExtendedDataSquare,
) (file.EdsFile, error) {
	tNow := time.Now()
	lock := s.stripLock.byDatahashAndHeight(datahash, height)
	lock.lock()
	defer lock.unlock()

	if datahash.IsEmptyRoot() {
		s.addEmptyHeight(height)
		return emptyFile, nil
	}

	// short circuit if file exists
	if has, _ := s.hasByHash(datahash); has {
		s.metrics.observePutExist(ctx)
		return s.getByHash(datahash)
	}

	if has, _ := s.hasByHeight(height); has {
		log.Warnw("put: file already exists by height, but not by hash",
			"height", height,
			"hash", datahash.String())
		s.metrics.observePutExist(ctx)
		return s.getByHeight(height)
	}

	path := s.basepath + hashsPath + datahash.String()
	file, err := file.CreateOdsFile(path, height, datahash, square)
	if err != nil {
		s.metrics.observePut(ctx, time.Since(tNow), square.Width(), true)
		return nil, fmt.Errorf("creating ODS file: %w", err)
	}

	// create hard link with height as name
	err = os.Link(path, s.basepath+heightsPath+fmt.Sprintf("%d", height))
	if err != nil {
		s.metrics.observePut(ctx, time.Since(tNow), square.Width(), true)
		return nil, fmt.Errorf("creating hard link: %w", err)
	}

	s.metrics.observePut(ctx, time.Since(tNow), square.Width(), false)

	// put in recent cache
	f, err := s.cache.First().GetOrLoad(ctx, height, edsLoader(file))
	if err != nil {
		return nil, fmt.Errorf("putting in cache: %w", err)
	}
	return f, nil
}

func (s *Store) GetByHash(ctx context.Context, datahash share.DataHash) (file.EdsFile, error) {
	if datahash.IsEmptyRoot() {
		return emptyFile, nil
	}
	lock := s.stripLock.byDatahash(datahash)
	lock.RLock()
	defer lock.RUnlock()

	tNow := time.Now()
	f, err := s.getByHash(datahash)
	s.metrics.observeGet(ctx, time.Since(tNow), err != nil)
	return f, err
}

func (s *Store) getByHash(datahash share.DataHash) (file.EdsFile, error) {
	if datahash.IsEmptyRoot() {
		return emptyFile, nil
	}

	path := s.basepath + hashsPath + datahash.String()
	odsFile, err := file.OpenOdsFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("opening ODS file: %w", err)
	}
	return odsFile, nil
}

func (s *Store) GetByHeight(ctx context.Context, height uint64) (file.EdsFile, error) {
	lock := s.stripLock.byHeight(height)
	lock.RLock()
	defer lock.RUnlock()

	tNow := time.Now()
	f, err := s.getByHeight(height)
	s.metrics.observeGet(ctx, time.Since(tNow), err != nil)
	return f, err
}

func (s *Store) getByHeight(height uint64) (file.EdsFile, error) {
	if s.isEmptyHeight(height) {
		return emptyFile, nil
	}

	f, err := s.cache.Get(height)
	if err == nil {
		return f, nil
	}

	path := s.basepath + heightsPath + fmt.Sprintf("%d", height)
	odsFile, err := file.OpenOdsFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("opening ODS file: %w", err)
	}
	return odsFile, nil
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
	path := s.basepath + hashsPath + datahash.String()
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
	if s.isEmptyHeight(height) {
		return true, nil
	}

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
	err := s.remove(height)
	s.metrics.observeRemove(ctx, time.Since(tNow), err != nil)
	return err
}

func (s *Store) remove(height uint64) error {
	// short circuit if file not exists
	f, err := s.getByHeight(height)
	if errors.Is(err, ErrNotFound) {
		return nil
	}

	hashStr := f.DataHash().String()
	if err = f.Close(); err != nil {
		return fmt.Errorf("closing file on removal: %w", err)
	}

	if err = s.cache.Remove(height); err != nil {
		return fmt.Errorf("removing from cache: %w", err)
	}

	heightPath := s.basepath + heightsPath + fmt.Sprintf("%d", height)
	if err = os.Remove(heightPath); err != nil {
		return fmt.Errorf("removing by height: %w", err)
	}

	hashPath := s.basepath + hashsPath + hashStr
	if err = os.Remove(hashPath); err != nil {
		return fmt.Errorf("removing by hash: %w", err)
	}
	return nil
}

func edsLoader(f file.EdsFile) cache.OpenFileFn {
	return func(ctx context.Context) (file.EdsFile, error) {
		return f, nil
	}
}

func (s *Store) openFileByHeight(height uint64) cache.OpenFileFn {
	return func(ctx context.Context) (file.EdsFile, error) {
		path := s.basepath + heightsPath + fmt.Sprintf("%d", height)
		f, err := file.OpenOdsFile(path)
		if err != nil {
			return nil, fmt.Errorf("opening ODS file: %w", err)
		}
		return f, nil
	}
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

func ensureFile(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		file, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("creating file: %w", err)
		}
		return file.Close()
	}
	if err != nil {
		return fmt.Errorf("checking file: %w", err)
	}
	if info.IsDir() {
		return errors.New("expected file, got a dir")
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

func (s *Store) storeEmptyHeights() error {
	file, err := os.OpenFile(s.basepath+emptyHeights, os.O_WRONLY, os.ModePerm)
	if err != nil {
		return fmt.Errorf("opening empty heights file: %w", err)
	}
	defer utils.CloseAndLog(log, "empty heights file", file)

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(s.emptyHeights); err != nil {
		return fmt.Errorf("encoding empty heights: %w", err)
	}

	return nil
}

func loadEmptyHeights(basepath string) (map[uint64]struct{}, error) {
	file, err := os.Open(basepath + emptyHeights)
	if err != nil {
		return nil, fmt.Errorf("opening empty heights file: %w", err)
	}
	defer utils.CloseAndLog(log, "empty heights file", file)

	emptyHeights := make(map[uint64]struct{})
	err = gob.NewDecoder(file).Decode(&emptyHeights)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decoding empty heights file: %w", err)
	}
	return emptyHeights, nil
}

func (s *Store) isEmptyHeight(height uint64) bool {
	s.emptyHeightsLock.RLock()
	defer s.emptyHeightsLock.RUnlock()
	_, ok := s.emptyHeights[height]
	return ok
}

func (s *Store) addEmptyHeight(height uint64) {
	s.emptyHeightsLock.Lock()
	defer s.emptyHeightsLock.Unlock()
	s.emptyHeights[height] = struct{}{}
}
