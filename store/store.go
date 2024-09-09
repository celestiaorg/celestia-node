package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/store/cache"
	"github.com/celestiaorg/celestia-node/store/file"
)

var log = logging.Logger("edsstore")

const (
	blocksPath     = "blocks"
	heightsPath    = blocksPath + "/heights"
	odsFileExt     = ".ods"
	q4FileExt      = ".q4"
	defaultDirPerm = 0o755
)

var ErrNotFound = errors.New("eds not found in store")

// Store is a storage for EDS files. It persists EDS files on disk in form of Q1Q4 files or ODS
// files. It provides methods to put, get and removeAll EDS files. It has two caches: recent eds cache
// and availability cache. Recent eds cache is used to cache recent blocks. Availability cache is
// used to cache blocks that are accessed by sample requests. Store is thread-safe.
type Store struct {
	// basepath is the root directory of the store
	basepath string
	// cache is used to cache recent blocks and blocks that are accessed frequently
	cache cache.Cache
	// stripedLocks is used to synchronize parallel operations
	stripLock *striplock
	metrics   *metrics
}

// NewStore creates a new EDS Store under the given basepath and datastore.
func NewStore(params *Parameters, basePath string) (*Store, error) {
	err := params.Validate()
	if err != nil {
		return nil, err
	}

	// ensure the blocks dir exists
	blocksDir := filepath.Join(basePath, blocksPath)
	if err := mkdir(blocksDir); err != nil {
		return nil, fmt.Errorf("ensuring blocks directory: %w", err)
	}

	// ensure the heights dir exists
	heightsDir := filepath.Join(basePath, heightsPath)
	if err := mkdir(heightsDir); err != nil {
		return nil, fmt.Errorf("ensuring heights directory: %w", err)
	}

	var recentCache cache.Cache = cache.NoopCache{}
	if params.RecentBlocksCacheSize > 0 {
		recentCache, err = cache.NewAccessorCache("recent", params.RecentBlocksCacheSize)
		if err != nil {
			return nil, fmt.Errorf("failed to create recent eds cache: %w", err)
		}
	}

	store := &Store{
		basepath:  basePath,
		cache:     recentCache,
		stripLock: newStripLock(1024),
	}

	if err := store.populateEmptyFile(); err != nil {
		return nil, fmt.Errorf("ensuring empty EDS: %w", err)
	}

	return store, nil
}

func (s *Store) Stop(context.Context) error {
	return s.metrics.close()
}

func (s *Store) Put(
	ctx context.Context,
	roots *share.AxisRoots,
	height uint64,
	square *rsmt2d.ExtendedDataSquare,
) error {
	datahash := share.DataHash(roots.Hash())
	// we don't need to store empty EDS, just link the height to the empty file
	if datahash.IsEmptyEDS() {
		lock := s.stripLock.byHeight(height)
		lock.Lock()
		err := s.linkHeight(datahash, height)
		lock.Unlock()
		return err
	}

	// put to cache before writing to make it accessible while write is happening
	accessor := &eds.Rsmt2D{ExtendedDataSquare: square}
	acc, err := s.cache.GetOrLoad(ctx, height, accessorLoader(accessor))
	if err != nil {
		log.Warnf("failed to put Accessor in the recent cache: %s", err)
	} else {
		// release the ref link to the accessor
		utils.CloseAndLog(log, "recent accessor", acc)
	}

	tNow := time.Now()
	lock := s.stripLock.byHashAndHeight(datahash, height)
	lock.lock()
	defer lock.unlock()

	exists, err := s.createFile(square, roots, height)
	if exists {
		s.metrics.observePutExist(ctx)
		return nil
	}
	if err != nil {
		s.metrics.observePut(ctx, time.Since(tNow), square.Width(), true)
		return fmt.Errorf("creating file: %w", err)
	}

	s.metrics.observePut(ctx, time.Since(tNow), square.Width(), false)
	return nil
}

func (s *Store) createFile(
	square *rsmt2d.ExtendedDataSquare,
	roots *share.AxisRoots,
	height uint64,
) (bool, error) {
	pathODS := s.hashToPath(roots.Hash(), odsFileExt)
	pathQ4 := s.hashToPath(roots.Hash(), q4FileExt)

	err := file.CreateODSQ4(pathODS, pathQ4, roots, square)
	if errors.Is(err, os.ErrExist) {
		// TODO(@Wondertan): Should we verify that the exist file is correct?
		return true, nil
	}
	if err != nil {
		// ensure we don't have partial writes if any operation fails
		removeErr := s.removeAll(height, roots.Hash())
		return false, errors.Join(
			fmt.Errorf("creating ODSQ4 file: %w", err),
			removeErr,
		)
	}

	// create hard link with height as name
	err = s.linkHeight(roots.Hash(), height)
	if err != nil {
		// ensure we don't have partial writes if any operation fails
		removeErr := s.removeAll(height, roots.Hash())
		return false, errors.Join(
			fmt.Errorf("hardlinking height: %w", err),
			removeErr,
		)
	}
	return false, nil
}

func (s *Store) linkHeight(datahash share.DataHash, height uint64) error {
	// create hard link with height as name
	pathOds := s.hashToPath(datahash, odsFileExt)
	linktoOds := s.heightToPath(height, odsFileExt)
	return link(pathOds, linktoOds)
}

// populateEmptyFile writes fresh empty EDS file on disk.
// It overrides existing empty file to ensure disk format is always consistent with the canonical
// in-mem representation.
func (s *Store) populateEmptyFile() error {
	pathOds := s.hashToPath(share.EmptyEDSDataHash(), odsFileExt)
	pathQ4 := s.hashToPath(share.EmptyEDSDataHash(), q4FileExt)

	err := errors.Join(remove(pathOds), remove(pathQ4))
	if err != nil {
		return fmt.Errorf("cleaning old empty EDS file: %w", err)
	}

	err = file.CreateODSQ4(pathOds, pathQ4, share.EmptyEDSRoots(), eds.EmptyAccessor.ExtendedDataSquare)
	if err != nil {
		return fmt.Errorf("creating fresh empty EDS file: %w", err)
	}

	return nil
}

func (s *Store) GetByHash(ctx context.Context, datahash share.DataHash) (eds.AccessorStreamer, error) {
	if datahash.IsEmptyEDS() {
		return eds.EmptyAccessor, nil
	}
	lock := s.stripLock.byHash(datahash)
	lock.RLock()
	defer lock.RUnlock()

	tNow := time.Now()
	f, err := s.getByHash(ctx, datahash)
	s.metrics.observeGet(ctx, time.Since(tNow), err != nil)
	return f, err
}

func (s *Store) getByHash(ctx context.Context, datahash share.DataHash) (eds.AccessorStreamer, error) {
	path := s.hashToPath(datahash, odsFileExt)
	return s.openAccessor(ctx, path)
}

func (s *Store) GetByHeight(ctx context.Context, height uint64) (eds.AccessorStreamer, error) {
	lock := s.stripLock.byHeight(height)
	lock.RLock()
	defer lock.RUnlock()

	tNow := time.Now()
	f, err := s.getByHeight(ctx, height)
	s.metrics.observeGet(ctx, time.Since(tNow), err != nil)
	return f, err
}

func (s *Store) getByHeight(ctx context.Context, height uint64) (eds.AccessorStreamer, error) {
	f, err := s.cache.Get(height)
	if err == nil {
		return f, nil
	}

	path := s.heightToPath(height, odsFileExt)
	return s.openAccessor(ctx, path)
}

// openAccessor opens ODSQ4 Accessor.
// It opens ODS file first, reads up its DataHash and constructs the path for Q4
// This done as Q4 is not indexed(hard-linked) and there is no other way to Q4 by height only.
func (s *Store) openAccessor(ctx context.Context, path string) (eds.AccessorStreamer, error) {
	ods, err := file.OpenODS(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open ODS: %w", err)
	}

	// read datahash from ODS and construct Q4 path
	datahash, err := ods.DataHash(ctx)
	if err != nil {
		utils.CloseAndLog(log, "open ods", ods)
		return nil, fmt.Errorf("reading datahash: %w", err)
	}
	pathQ4 := s.hashToPath(datahash, q4FileExt)
	odsQ4 := file.ODSWithQ4(ods, pathQ4)
	return wrapAccessor(odsQ4), nil
}

func (s *Store) HasByHash(ctx context.Context, datahash share.DataHash) (bool, error) {
	if datahash.IsEmptyEDS() {
		return true, nil
	}

	lock := s.stripLock.byHash(datahash)
	lock.RLock()
	defer lock.RUnlock()

	tNow := time.Now()
	exist, err := s.hasByHash(datahash)
	s.metrics.observeHas(ctx, time.Since(tNow), err != nil)
	return exist, err
}

func (s *Store) hasByHash(datahash share.DataHash) (bool, error) {
	// For now, we assume that if ODS exists, the Q4 exists as well.
	path := s.hashToPath(datahash, odsFileExt)
	return exists(path)
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
	acc, err := s.cache.Get(height)
	if err == nil {
		utils.CloseAndLog(log, "accessor", acc)
		return true, nil
	}

	// For now, we assume that if ODS exists, the Q4 exists as well.
	pathODS := s.heightToPath(height, odsFileExt)
	return exists(pathODS)
}

func (s *Store) RemoveAll(ctx context.Context, height uint64, datahash share.DataHash) error {
	lock := s.stripLock.byHashAndHeight(datahash, height)
	lock.lock()
	defer lock.unlock()

	tNow := time.Now()
	err := s.removeAll(height, datahash)
	s.metrics.observeRemoveAll(ctx, time.Since(tNow), err != nil)
	return err
}

func (s *Store) removeAll(height uint64, datahash share.DataHash) error {
	if err := s.removeODS(height, datahash); err != nil {
		return fmt.Errorf("removing ODS: %w", err)
	}
	if err := s.removeQ4(height, datahash); err != nil {
		return fmt.Errorf("removing Q4: %w", err)
	}
	return nil
}

func (s *Store) removeODS(height uint64, datahash share.DataHash) error {
	if err := s.cache.Remove(height); err != nil {
		return fmt.Errorf("removing from cache: %w", err)
	}

	pathLink := s.heightToPath(height, odsFileExt)
	if err := remove(pathLink); err != nil {
		return fmt.Errorf("removing hardlink: %w", err)
	}

	// if datahash is empty, we don't need to remove the ODS file, only the hardlink
	if datahash.IsEmptyEDS() {
		return nil
	}

	pathODS := s.hashToPath(datahash, odsFileExt)
	if err := remove(pathODS); err != nil {
		return fmt.Errorf("removing ODS file: %w", err)
	}
	return nil
}

func (s *Store) RemoveQ4(ctx context.Context, height uint64, datahash share.DataHash) error {
	lock := s.stripLock.byHashAndHeight(datahash, height)
	lock.lock()
	defer lock.unlock()

	tNow := time.Now()
	err := s.removeQ4(height, datahash)
	s.metrics.observeRemoveQ4(ctx, time.Since(tNow), err != nil)
	return err
}

func (s *Store) removeQ4(height uint64, datahash share.DataHash) error {
	// if datahash is empty, we don't need to remove the Q4 file
	if datahash.IsEmptyEDS() {
		return nil
	}

	if err := s.cache.Remove(height); err != nil {
		return fmt.Errorf("removing from cache: %w", err)
	}

	// remove Q4 file
	pathQ4File := s.hashToPath(datahash, q4FileExt)
	if err := remove(pathQ4File); err != nil {
		return fmt.Errorf("removing Q4 file: %w", err)
	}
	return nil
}

func (s *Store) hashToPath(datahash share.DataHash, ext string) string {
	return filepath.Join(s.basepath, blocksPath, datahash.String()) + ext
}

func (s *Store) heightToPath(height uint64, ext string) string {
	return filepath.Join(s.basepath, heightsPath, strconv.Itoa(int(height))) + ext
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

func mkdir(path string) error {
	err := os.Mkdir(path, defaultDirPerm)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("making directory '%s': %w", path, err)
	}

	return nil
}

func link(filepath, linkpath string) error {
	err := os.Link(filepath, linkpath)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("creating hardlink (%s -> %s): %w", filepath, linkpath, err)
	}
	return nil
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, fmt.Errorf("checking file existence '%s': %w", path, err)
	}
}

func remove(path string) error {
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing file '%s': %w", path, err)
	}
	return nil
}
