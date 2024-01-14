package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/store/cache"
	"github.com/celestiaorg/celestia-node/share/store/file"
	"github.com/celestiaorg/rsmt2d"
	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel"
	"os"
)

var (
	log    = logging.Logger("share/eds")
	tracer = otel.Tracer("share/eds")
)

const (
	blocksPath  = "/blocks/"
	heightsPath = "/heights/"
)

var ErrNotFound = errors.New("eds not found in store")

// Store maintains (via DAGStore) a top-level index enabling granular and efficient random access to
// every share and/or Merkle proof over every registered CARv1 file. The EDSStore provides a custom
// blockstore interface implementation to achieve access. The main use-case is randomized sampling
// over the whole chain of EDS block data and getting data by namespace.
type Store struct {
	cancel context.CancelFunc

	basepath string

	// cache is used to cache recent blocks and blocks that are accessed frequently
	cache *cache.DoubleCache

	//TODO: maintain in-memory missing files index / bloom-filter to fast return for not stored files.

	// stripedLocks is used to synchronize parallel operations
	stripLock *striplock

	//metrics *metrics
}

// NewStore creates a new EDS Store under the given basepath and datastore.
func NewStore(params *Parameters, basePath string) (*Store, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	//TODO: acquire DirectoryLock store lockGuard

	recentBlocksCache, err := cache.NewFileCache("recent", params.RecentBlocksCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create recent blocks cache: %w", err)
	}

	blockstoreCache, err := cache.NewFileCache("blockstore", params.BlockstoreCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockstore cache: %w", err)
	}

	dcache := cache.NewDoubleCache(recentBlocksCache, blockstoreCache)

	store := &Store{
		basepath:  basePath,
		cache:     dcache,
		stripLock: newStripLock(1024),
		//metrics:   newMetrics(),
	}
	return store, nil
}

func (s *Store) Put(
	ctx context.Context,
	datahash share.DataHash,
	height uint64,
	square *rsmt2d.ExtendedDataSquare,
) (file.EdsFile, error) {
	lock := s.stripLock.byDatahashAndHeight(datahash, height)
	lock.lock()
	defer lock.unlock()

	path := s.basepath + blocksPath + datahash.String()
	odsFile, err := file.CreateOdsFile(path, height, datahash, square)
	if err != nil {
		return nil, fmt.Errorf("writing ODS file: %w", err)
	}

	// create hard link with height as name
	err = os.Link(path, s.basepath+heightsPath+fmt.Sprintf("%d", height))
	if err != nil {
		return nil, fmt.Errorf("creating hard link: %w", err)
	}

	// put in recent cache
	f, err := s.cache.First().GetOrLoad(ctx, cache.Key{Height: height}, wrapWithCache(odsFile))
	if err != nil {
		return nil, fmt.Errorf("putting in cache: %w", err)
	}
	return f, nil
}

func (s *Store) GetByHash(_ context.Context, datahash share.DataHash) (file.EdsFile, error) {
	lock := s.stripLock.byDatahash(datahash)
	lock.RLock()
	defer lock.RUnlock()

	f, err := s.cache.Get(cache.Key{Datahash: datahash})
	if err == nil {
		return f, nil
	}

	path := s.basepath + blocksPath + datahash.String()
	odsFile, err := file.OpenOdsFile(path)
	if err != nil {
		return nil, fmt.Errorf("opening ODS file: %w", err)
	}
	return odsFile, nil
}

func (s *Store) GetByHeight(_ context.Context, height uint64) (file.EdsFile, error) {
	lock := s.stripLock.byHeight(height)
	lock.RLock()
	defer lock.RUnlock()

	f, err := s.cache.Get(cache.Key{Height: height})
	if err == nil {
		return f, nil
	}

	path := s.basepath + heightsPath + fmt.Sprintf("%d", height)
	odsFile, err := file.OpenOdsFile(path)
	if err != nil {
		return nil, fmt.Errorf("opening ODS file: %w", err)
	}
	return odsFile, nil
}

func (s *Store) HasByHash(_ context.Context, datahash share.DataHash) (bool, error) {
	lock := s.stripLock.byDatahash(datahash)
	lock.RLock()
	defer lock.RUnlock()

	_, err := s.cache.Get(cache.Key{Datahash: datahash})
	if err == nil {
		return true, nil
	}

	path := s.basepath + blocksPath + datahash.String()
	_, err = file.OpenOdsFile(path)
	if err != nil {
		return false, fmt.Errorf("opening ODS file: %w", err)
	}
	return true, nil
}

func (s *Store) HasByHeight(_ context.Context, height uint64) (bool, error) {
	lock := s.stripLock.byHeight(height)
	lock.RLock()
	defer lock.RUnlock()

	_, err := s.cache.Get(cache.Key{Height: height})
	if err == nil {
		return true, nil
	}

	path := s.basepath + heightsPath + fmt.Sprintf("%d", height)
	_, err = file.OpenOdsFile(path)
	if err != nil {
		return false, fmt.Errorf("opening ODS file: %w", err)
	}
	return true, nil
}

func wrapWithCache(f file.EdsFile) cache.OpenFileFn {
	return func(ctx context.Context) (cache.Key, file.EdsFile, error) {
		f := file.NewCacheFile(f)
		key := cache.Key{
			Datahash: f.DataHash(),
			Height:   f.Height(),
		}
		return key, f, nil
	}
}

func (s *Store) openFileByHeight(height uint64) cache.OpenFileFn {
	return func(ctx context.Context) (cache.Key, file.EdsFile, error) {
		path := s.basepath + heightsPath + fmt.Sprintf("%d", height)
		f, err := file.OpenOdsFile(path)
		if err != nil {
			return cache.Key{}, nil, fmt.Errorf("opening ODS file: %w", err)
		}
		key := cache.Key{
			Datahash: f.DataHash(),
			Height:   height,
		}
		return key, file.NewCacheFile(f), nil
	}
}

func (s *Store) openFileByDatahash(datahash share.DataHash) cache.OpenFileFn {
	return func(ctx context.Context) (cache.Key, file.EdsFile, error) {
		path := s.basepath + blocksPath + datahash.String()
		f, err := file.OpenOdsFile(path)
		if err != nil {
			return cache.Key{}, nil, fmt.Errorf("opening ODS file: %w", err)
		}
		key := cache.Key{
			Datahash: f.DataHash(),
			Height:   f.Height(),
		}
		return key, file.NewCacheFile(f), nil
	}
}

func (s *Store) Close() error {
	panic("implement me")
	return nil
}
