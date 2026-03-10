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
	"github.com/celestiaorg/celestia-node/share/eds"
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

// AccessorGetter abstracts storage system that indexes and manages multiple eds.AccessorGetter by
// network height.
type AccessorGetter interface {
	// GetByHeight returns an Accessor by its height.
	GetByHeight(ctx context.Context, height uint64) (eds.AccessorStreamer, error)
	// HasByHeight reports whether an Accessor for the height exists.
	HasByHeight(ctx context.Context, height uint64) (bool, error)
}

// Store is a storage for EDS files. It persists EDS files on disk in form of Q1Q4 files or ODS
// files. It provides methods to put, get and remove EDS files. It has two caches: recent eds cache
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

func (s *Store) PutODSQ4(
	ctx context.Context,
	roots *share.AxisRoots,
	height uint64,
	square *rsmt2d.ExtendedDataSquare,
) error {
	return s.put(ctx, roots, height, square, true)
}

func (s *Store) PutODS(
	ctx context.Context,
	roots *share.AxisRoots,
	height uint64,
	square *rsmt2d.ExtendedDataSquare,
) error {
	return s.put(ctx, roots, height, square, false)
}

func (s *Store) put(
	ctx context.Context,
	roots *share.AxisRoots,
	height uint64,
	square *rsmt2d.ExtendedDataSquare,
	writeQ4 bool,
) error {
	datahash := share.DataHash(roots.Hash())
	// we don't need to store empty EDS, just link the height to the empty file
	if datahash.IsEmptyEDS() {
		lock := s.stripLock.byHeight(height)
		lock.Lock()
		defer lock.Unlock()
		err := s.linkHeight(datahash, height)
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}

	// ensure the blocks hash directory exists
	blocksHashDir := filepath.Join(s.basepath, blocksPath, datahash.String()[:2])
	if err := mkdir(blocksHashDir); err != nil {
		return fmt.Errorf("ensuring blocks hash directory: %w", err)
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

	var exists bool
	if writeQ4 {
		exists, err = s.createODSQ4File(square, roots, height)
	} else {
		exists, err = s.createODSFile(square, roots, height)
	}

	if exists {
		s.metrics.observePutExist(ctx)
		return nil
	}
	if err != nil {
		s.metrics.observePut(ctx, time.Since(tNow), square.Width(), writeQ4, true)
		return fmt.Errorf("creating file: %w", err)
	}

	s.metrics.observePut(ctx, time.Since(tNow), square.Width(), writeQ4, false)
	return nil
}

func (s *Store) createODSQ4File(
	square *rsmt2d.ExtendedDataSquare,
	roots *share.AxisRoots,
	height uint64,
) (bool, error) {
	pathODS := s.hashToPath(roots.Hash(), odsFileExt)
	pathQ4 := s.hashToPath(roots.Hash(), q4FileExt)

	err := file.CreateODSQ4(pathODS, pathQ4, roots, square)
	if err != nil && !errors.Is(err, os.ErrExist) {
		// ensure we don't have partial writes if any operation fails
		removeErr := s.removeODSQ4(height, roots.Hash())
		return false, errors.Join(
			fmt.Errorf("creating ODSQ4 file: %w", err),
			removeErr,
		)
	}

	// if file already exists, check if it's corrupted
	if errors.Is(err, os.ErrExist) {
		err = s.validateAndRecoverODSQ4(square, roots, height, pathODS, pathQ4)
		if err != nil {
			return false, err
		}
	}

	// create hard link with height as name
	err = s.linkHeight(roots.Hash(), height)
	// if both file and link exist, we consider it as success
	if errors.Is(err, os.ErrExist) {
		return true, nil
	}
	if err != nil {
		// ensure we don't have partial writes if any operation fails
		removeErr := s.removeODSQ4(height, roots.Hash())
		return false, errors.Join(
			fmt.Errorf("hardlinking height: %w", err),
			removeErr,
		)
	}
	return false, nil
}

func (s *Store) validateAndRecoverODSQ4(
	square *rsmt2d.ExtendedDataSquare,
	roots *share.AxisRoots,
	height uint64,
	pathODS, pathQ4 string,
) error {
	// Validate the size of the file to ensure it's not corrupted
	err := file.ValidateODSQ4Size(pathODS, pathQ4, square)
	if err == nil {
		return nil
	}
	log.Warnf("ODSQ4 file with height %d is corrupted, recovering", height)
	err = s.removeODSQ4(height, roots.Hash())
	if err != nil {
		return fmt.Errorf("removing corrupted ODSQ4 file: %w", err)
	}
	err = file.CreateODSQ4(pathODS, pathQ4, roots, square)
	if err != nil {
		return fmt.Errorf("recreating ODSQ4 file: %w", err)
	}
	return nil
}

func (s *Store) createODSFile(
	square *rsmt2d.ExtendedDataSquare,
	roots *share.AxisRoots,
	height uint64,
) (bool, error) {
	pathODS := s.hashToPath(roots.Hash(), odsFileExt)
	err := file.CreateODS(pathODS, roots, square)
	if err != nil && !errors.Is(err, os.ErrExist) {
		// ensure we don't have partial writes if any operation fails
		removeErr := s.removeODS(height, roots.Hash())
		return false, errors.Join(
			fmt.Errorf("creating ODS file: %w", err),
			removeErr,
		)
	}

	// if file already exists, check if it's corrupted
	if errors.Is(err, os.ErrExist) {
		// Validate the size of the file to ensure it's not corrupted
		err = s.validateAndRecoverODS(square, roots, height, pathODS)
		if err != nil {
			return false, err
		}
	}

	// create hard link with height as name
	err = s.linkHeight(roots.Hash(), height)
	// if both file and link exist, we consider it as success
	if errors.Is(err, os.ErrExist) {
		return true, nil
	}
	if err != nil {
		// ensure we don't have partial writes if any operation fails
		removeErr := s.removeODS(height, roots.Hash())
		return false, errors.Join(
			fmt.Errorf("hardlinking height: %w", err),
			removeErr,
		)
	}
	return false, nil
}

func (s *Store) validateAndRecoverODS(
	square *rsmt2d.ExtendedDataSquare,
	roots *share.AxisRoots,
	height uint64,
	pathODS string,
) error {
	// Validate the size of the file to ensure it's not corrupted
	err := file.ValidateODSSize(pathODS, square)
	if err == nil {
		return nil
	}
	log.Warnf("ODS file with height %d is corrupted, recovering", height)
	err = s.removeODS(height, roots.Hash())
	if err != nil {
		return fmt.Errorf("removing corrupted ODS file: %w", err)
	}
	err = file.CreateODS(pathODS, roots, square)
	if err != nil {
		return fmt.Errorf("recreating ODS file: %w", err)
	}
	return nil
}

func (s *Store) linkHeight(datahash share.DataHash, height uint64) error {
	linktoOds := s.heightToPath(height, odsFileExt)
	// empty EDS is always symlinked, because there is limited number of hardlinks
	// for the same file in some filesystems (ext4)
	pathOds := s.hashToRelativePath(datahash, odsFileExt)
	return symlink(pathOds, linktoOds)
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

	// ensure the blocks hash directory exists
	blocksHashDir := filepath.Join(s.basepath, blocksPath, share.EmptyEDSDataHash().String()[:2])
	if err := mkdir(blocksHashDir); err != nil {
		return fmt.Errorf("ensuring blocks hash directory: %w", err)
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
	if s.cache.Has(height) {
		return true, nil
	}

	// For now, we assume that if ODS exists, the Q4 exists as well.
	pathODS := s.heightToPath(height, odsFileExt)
	return exists(pathODS)
}

func (s *Store) HasQ4ByHash(_ context.Context, datahash share.DataHash) (bool, error) {
	lock := s.stripLock.byHash(datahash)
	lock.RLock()
	defer lock.RUnlock()

	pathQ4File := s.hashToPath(datahash, q4FileExt)
	return exists(pathQ4File)
}

func (s *Store) RemoveODSQ4(ctx context.Context, height uint64, datahash share.DataHash) error {
	lock := s.stripLock.byHashAndHeight(datahash, height)
	lock.lock()
	defer lock.unlock()

	tNow := time.Now()
	err := s.removeODSQ4(height, datahash)
	s.metrics.observeRemoveODSQ4(ctx, time.Since(tNow), err != nil)
	return err
}

func (s *Store) removeODSQ4(height uint64, datahash share.DataHash) error {
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
	return filepath.Join(s.basepath, blocksPath, datahash.String()[:2], datahash.String()) + ext
}

func (s *Store) hashToRelativePath(datahash share.DataHash, ext string) string {
	return filepath.Join("..", datahash.String()[:2], datahash.String()) + ext
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

func symlink(filepath, linkpath string) error {
	err := os.Symlink(filepath, linkpath)
	if err != nil {
		return fmt.Errorf("creating symlink (%s -> %s): %w", filepath, linkpath, err)
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
