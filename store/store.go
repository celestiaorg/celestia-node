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

var (
	log = logging.Logger("share/eds")
)

const (
	blocksPath     = "blocks"
	heightsPath    = blocksPath + "/heights"
	odsFileExt     = ".ods"
	q4FileExt      = ".q4"
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

func (s *Store) Close() error {
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
		return false, errors.Join(
			fmt.Errorf("creating ODSQ4 file: %w", err),
			// ensure we don't have partial writes
			remove(pathODS),
			remove(pathQ4),
		)
	}

	// create hard link with height as name
	err = s.linkHeight(roots.Hash(), height)
	if err != nil {
		// ensure we don't have partial writes if any operation fails
		return false, errors.Join(
			fmt.Errorf("hardlinking height: %w", err),
			remove(pathODS),
			remove(pathQ4),
			s.removeLink(height),
		)
	}
	return false, nil
}

func (s *Store) linkHeight(datahash share.DataHash, height uint64) error {
	// create hard link with height as name
	pathOds := s.hashToPath(datahash, odsFileExt)
	linktoOds := s.heightToPath(height, odsFileExt)
	pathQ4 := s.hashToPath(datahash, q4FileExt)
	linktoQ4 := s.heightToPath(height, q4FileExt)
	return errors.Join(
		link(pathOds, linktoOds),
		link(pathQ4, linktoQ4),
	)
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
	f, err := s.getByHash(datahash)
	s.metrics.observeGet(ctx, time.Since(tNow), err != nil)
	return f, err
}

func (s *Store) getByHash(datahash share.DataHash) (eds.AccessorStreamer, error) {
	path := s.hashToPath(datahash, "")
	return s.openODSQ4(path)
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

	path := s.heightToPath(height, "")
	return s.openODSQ4(path)
}

// openODSQ4 opens ODSQ4 Accessor.
// It opens ODS file first, reads up its DataHash and constructs the path for Q4
// This done as Q4 is not indexed(hardlinked) and there is no other way to Q4 by height only.
func (s *Store) openODSQ4(path string) (eds.AccessorStreamer, error) {
	odsq4, err := file.OpenODSQ4(path+odsFileExt, path+q4FileExt)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("opening ODSQ4: %w", err)
	}

	return wrapAccessor(odsq4), nil
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

func (s *Store) Remove(ctx context.Context, height uint64, datahash share.DataHash) error {
	tNow := time.Now()
	err := s.remove(height, datahash)
	s.metrics.observeRemove(ctx, time.Since(tNow), err != nil)
	return err
}

func (s *Store) remove(height uint64, datahash share.DataHash) error {
	lock := s.stripLock.byHeight(height)
	lock.Lock()
	if err := s.removeLink(height); err != nil {
		return fmt.Errorf("removing link: %w", err)
	}
	lock.Unlock()

	dlock := s.stripLock.byHash(datahash)
	dlock.Lock()
	defer dlock.Unlock()
	if err := s.removeFile(datahash); err != nil {
		return fmt.Errorf("removing file: %w", err)
	}
	return nil
}

func (s *Store) removeLink(height uint64) error {
	if err := s.cache.Remove(height); err != nil {
		return fmt.Errorf("removing from cache: %w", err)
	}

	pathODS := s.heightToPath(height, odsFileExt)
	pathQ4 := s.heightToPath(height, q4FileExt)
	return errors.Join(
		remove(pathODS),
		remove(pathQ4),
	)
}

func (s *Store) removeFile(hash share.DataHash) error {
	// we don't need to remove the empty file, it should always be there
	if hash.IsEmptyEDS() {
		return nil
	}

	pathODS := s.hashToPath(hash, odsFileExt)
	pathQ4 := s.hashToPath(hash, q4FileExt)
	return errors.Join(
		remove(pathODS),
		remove(pathQ4),
	)
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
