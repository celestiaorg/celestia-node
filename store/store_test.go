package store

import (
	"context"
	"os"
	"path"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/store/cache"
	"github.com/celestiaorg/celestia-node/store/file"
)

func TestEDSStore(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	height := atomic.Uint64{}
	height.Store(1)

	putTests := []struct {
		name               string
		newEds             func(t testing.TB) (*rsmt2d.ExtendedDataSquare, *share.AxisRoots)
		putFn              func(*Store) putFunc
		addedFiles         int
		addedLinks         int
		filesAfterRemoveQ4 int
	}{
		{
			name:   "ODS, non empty eds",
			newEds: randomEDS,
			putFn: func(store *Store) putFunc {
				return store.PutODS
			},
			addedFiles:         1,
			addedLinks:         1,
			filesAfterRemoveQ4: 1,
		},
		{
			name: "ODS, empty eds",
			newEds: func(t testing.TB) (*rsmt2d.ExtendedDataSquare, *share.AxisRoots) {
				return share.EmptyEDS(), share.EmptyEDSRoots()
			},
			putFn: func(store *Store) putFunc {
				return store.PutODS
			},
			addedFiles:         0,
			addedLinks:         1,
			filesAfterRemoveQ4: 0,
		},
		{
			name:   "ODSQ4, non empty eds",
			newEds: randomEDS,
			putFn: func(store *Store) putFunc {
				return store.PutODSQ4
			},
			addedFiles:         2,
			addedLinks:         1,
			filesAfterRemoveQ4: 1,
		},
		{
			name: "ODSQ4, empty eds",
			newEds: func(t testing.TB) (*rsmt2d.ExtendedDataSquare, *share.AxisRoots) {
				return share.EmptyEDS(), share.EmptyEDSRoots()
			},
			putFn: func(store *Store) putFunc {
				return store.PutODSQ4
			},
			addedFiles:         0,
			addedLinks:         1,
			filesAfterRemoveQ4: 0,
		},
	}

	for _, test := range putTests {
		t.Run(test.name, func(t *testing.T) {
			t.Run("Put", func(t *testing.T) {
				dir := t.TempDir()
				edsStore, err := NewStore(paramsNoCache(), dir)
				require.NoError(t, err)

				eds, roots := test.newEds(t)
				height := height.Add(1)

				err = test.putFn(edsStore)(ctx, roots, height, eds)
				require.NoError(t, err)

				// file should exist in the store
				hasByHashAndHeight(t, edsStore, ctx, roots.Hash(), height, true, true)

				// block folder should contain the correct amount of files and links
				ensureAmountFileAndLinks(t, dir, test.addedFiles, test.addedLinks)
			})

			t.Run("Cached after Put", func(t *testing.T) {
				eds, roots := test.newEds(t)
				if share.DataHash(roots.Hash()).IsEmptyEDS() {
					// skip test, empty eds is not cached after put
					t.Skip()
				}

				edsStore, err := NewStore(DefaultParameters(), t.TempDir())
				require.NoError(t, err)

				height := height.Add(1)
				err = test.putFn(edsStore)(ctx, roots, height, eds)
				require.NoError(t, err)

				// non-empty file should be cached after put
				f, err := edsStore.cache.Get(height)
				require.NoError(t, err)
				require.NoError(t, f.Close())

				// check that cached file is the same eds
				fromFile, err := f.Shares(ctx)
				require.NoError(t, err)
				require.NoError(t, f.Close())
				expected := eds.FlattenedODS()
				require.Equal(t, expected, fromFile)
			})

			t.Run("Second Put should be noop", func(t *testing.T) {
				dir := t.TempDir()
				edsStore, err := NewStore(paramsNoCache(), dir)
				require.NoError(t, err)

				eds, roots := test.newEds(t)
				height := height.Add(1)

				err = test.putFn(edsStore)(ctx, roots, height, eds)
				require.NoError(t, err)
				// ensure correct amount of files and links are written
				ensureAmountFileAndLinks(t, dir, test.addedFiles, test.addedLinks)

				err = test.putFn(edsStore)(ctx, roots, height, eds)
				require.NoError(t, err)

				// ensure no new files or links are written
				ensureAmountFileAndLinks(t, dir, test.addedFiles, test.addedLinks)
			})

			t.Run("Second Put after partial write", func(t *testing.T) {
				dir := t.TempDir()
				edsStore, err := NewStore(paramsNoCache(), dir)
				require.NoError(t, err)

				eds, roots := test.newEds(t)
				height := height.Add(1)

				err = test.putFn(edsStore)(ctx, roots, height, eds)
				require.NoError(t, err)
				// remove link
				pathLink := edsStore.heightToPath(height, odsFileExt)
				err = remove(pathLink)
				require.NoError(t, err)
				ensureAmountLinks(t, dir, 0)

				// put should write the missing link
				err = test.putFn(edsStore)(ctx, roots, height, eds)
				require.NoError(t, err)
				ensureAmountLinks(t, dir, test.addedLinks)
			})

			t.Run("RemoveODSQ4", func(t *testing.T) {
				dir := t.TempDir()
				edsStore, err := NewStore(DefaultParameters(), dir)
				require.NoError(t, err)

				eds, roots := test.newEds(t)
				height := height.Add(1)
				err = test.putFn(edsStore)(ctx, roots, height, eds)
				require.NoError(t, err)
				ensureAmountFileAndLinks(t, dir, test.addedFiles, test.addedLinks)

				hash := share.DataHash(roots.Hash())
				err = edsStore.RemoveODSQ4(ctx, height, hash)
				require.NoError(t, err)

				// file should be removed from cache
				_, err = edsStore.cache.Get(height)
				require.ErrorIs(t, err, cache.ErrCacheMiss)

				// empty file should be accessible by hash, non-empty file should not
				hasByHash := hash.IsEmptyEDS()
				// all files should not be accessible by height
				hasByHashAndHeight(t, edsStore, ctx, hash, height, hasByHash, false)

				// ensure all files and links are removed
				ensureAmountFileAndLinks(t, dir, 0, 0)
			})

			t.Run("RemoveQ4", func(t *testing.T) {
				dir := t.TempDir()
				edsStore, err := NewStore(DefaultParameters(), dir)
				require.NoError(t, err)

				eds, roots := test.newEds(t)
				height := height.Add(1)
				err = test.putFn(edsStore)(ctx, roots, height, eds)
				require.NoError(t, err)
				ensureAmountFileAndLinks(t, dir, test.addedFiles, test.addedLinks)

				hash := share.DataHash(roots.Hash())
				err = edsStore.RemoveQ4(ctx, height, hash)
				require.NoError(t, err)

				// file should be removed from cache
				_, err = edsStore.cache.Get(height)
				require.ErrorIs(t, err, cache.ErrCacheMiss)

				// ods file should still be accessible by hash and height
				hasByHashAndHeight(t, edsStore, ctx, hash, height, true, true)

				// ensure ods file and link are not removed
				ensureAmountFileAndLinks(t, dir, test.filesAfterRemoveQ4, test.addedLinks)
			})

			t.Run("GetByHeight", func(t *testing.T) {
				dir := t.TempDir()
				edsStore, err := NewStore(DefaultParameters(), dir)
				require.NoError(t, err)

				eds, roots := test.newEds(t)
				height := height.Add(1)
				err = test.putFn(edsStore)(ctx, roots, height, eds)
				require.NoError(t, err)
				ensureAmountFileAndLinks(t, dir, test.addedFiles, test.addedLinks)

				f, err := edsStore.GetByHeight(ctx, height)
				require.NoError(t, err)

				// check that file is the same eds
				fromFile, err := f.Shares(ctx)
				require.NoError(t, err)
				require.NoError(t, f.Close())
				expected := eds.FlattenedODS()
				require.Equal(t, expected, fromFile)
			})

			t.Run("GetByHash", func(t *testing.T) {
				dir := t.TempDir()
				edsStore, err := NewStore(DefaultParameters(), dir)
				require.NoError(t, err)

				eds, roots := test.newEds(t)
				height := height.Add(1)
				err = test.putFn(edsStore)(ctx, roots, height, eds)
				require.NoError(t, err)
				ensureAmountFileAndLinks(t, dir, test.addedFiles, test.addedLinks)

				f, err := edsStore.GetByHash(ctx, roots.Hash())
				require.NoError(t, err)

				// check that cached file is the same eds
				fromFile, err := f.Shares(ctx)
				require.NoError(t, err)
				require.NoError(t, f.Close())
				expected := eds.FlattenedODS()
				require.Equal(t, expected, fromFile)
			})
		})
	}

	t.Run("Does not exist", func(t *testing.T) {
		dir := t.TempDir()
		edsStore, err := NewStore(paramsNoCache(), dir)
		require.NoError(t, err)

		_, roots := randomEDS(t)
		// file does not exist
		hasByHashAndHeight(t, edsStore, ctx, roots.Hash(), 1, false, false)

		_, err = edsStore.GetByHeight(ctx, 1)
		require.ErrorIs(t, err, ErrNotFound)

		_, err = edsStore.GetByHash(ctx, roots.Hash())
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("empty EDS returned by hash", func(t *testing.T) {
		dir := t.TempDir()
		edsStore, err := NewStore(paramsNoCache(), dir)
		require.NoError(t, err)

		// assert that the empty file exists
		roots := share.EmptyEDSRoots()
		has, err := edsStore.HasByHash(ctx, roots.Hash())
		require.NoError(t, err)
		require.True(t, has)

		// assert that the empty file is, in fact, empty
		f, err := edsStore.GetByHash(ctx, roots.Hash())
		require.NoError(t, err)
		hash, err := f.DataHash(ctx)
		require.NoError(t, err)
		require.True(t, hash.IsEmptyEDS())
	})

	t.Run("reopen", func(t *testing.T) {
		dir := t.TempDir()
		edsStore, err := NewStore(paramsNoCache(), dir)
		require.NoError(t, err)

		// tests that store can be reopened
		err = edsStore.Stop(ctx)
		require.NoError(t, err)

		_, err = NewStore(paramsNoCache(), dir)
		require.NoError(t, err)
	})

	t.Run("recover ODS", func(t *testing.T) {
		dir := t.TempDir()
		edsStore, err := NewStore(paramsNoCache(), dir)
		require.NoError(t, err)

		eds, roots := randomEDS(t)
		height := height.Add(1)
		err = edsStore.PutODS(ctx, roots, height, eds)
		require.NoError(t, err)

		// corrupt ODS file
		pathODS := edsStore.heightToPath(height, odsFileExt)
		err = corruptFile(pathODS)
		require.NoError(t, err)

		// check if file is corrupted
		err = file.ValidateODSSize(pathODS, eds)
		require.Error(t, err)

		// second put should recover the file
		err = edsStore.PutODS(ctx, roots, height, eds)
		require.NoError(t, err)

		// check if file is recovered
		err = file.ValidateODSSize(pathODS, eds)
		require.NoError(t, err)
	})

	t.Run("recover ODSQ4", func(t *testing.T) {
		dir := t.TempDir()
		edsStore, err := NewStore(paramsNoCache(), dir)
		require.NoError(t, err)

		t.Run("corrupt ODS file", func(t *testing.T) {
			eds, roots := randomEDS(t)
			height := height.Add(1)
			err = edsStore.PutODSQ4(ctx, roots, height, eds)
			require.NoError(t, err)

			// corrupt ODS file
			pathODS := edsStore.heightToPath(height, odsFileExt)
			err := corruptFile(pathODS)
			require.NoError(t, err)

			// check if file is corrupted
			pathQ4 := edsStore.hashToPath(roots.Hash(), q4FileExt)
			err = file.ValidateODSQ4Size(pathODS, pathQ4, eds)
			require.Error(t, err)

			// second put should recover the file
			err = edsStore.PutODSQ4(ctx, roots, height, eds)
			require.NoError(t, err)

			// check if file is recovered
			err = file.ValidateODSQ4Size(pathODS, pathQ4, eds)
			require.NoError(t, err)
		})

		t.Run("corrupt Q4 file", func(t *testing.T) {
			eds, roots := randomEDS(t)
			height := height.Add(1)
			err = edsStore.PutODSQ4(ctx, roots, height, eds)
			require.NoError(t, err)

			// corrupt Q4 file
			pathQ4 := edsStore.hashToPath(roots.Hash(), q4FileExt)
			err := corruptFile(pathQ4)
			require.NoError(t, err)

			// check if file is corrupted
			pathODS := edsStore.heightToPath(height, odsFileExt)
			err = file.ValidateODSQ4Size(pathODS, pathQ4, eds)
			require.Error(t, err)

			// second put should recover the file
			err = edsStore.PutODSQ4(ctx, roots, height, eds)
			require.NoError(t, err)

			// check if file is recovered
			err = file.ValidateODSQ4Size(pathODS, pathQ4, eds)
			require.NoError(t, err)
		})
	})
}

func corruptFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	return file.Truncate(info.Size() - 1)
}

func BenchmarkStore(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	b.Cleanup(cancel)

	eds := edstest.RandEDS(b, 128)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(b, err)

	// BenchmarkStore/put_128-16         	     186	   6623266 ns/op
	b.Run("put 128", func(b *testing.B) {
		edsStore, err := NewStore(paramsNoCache(), b.TempDir())
		require.NoError(b, err)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			roots := edstest.RandomAxisRoots(b, 1)
			_ = edsStore.PutODSQ4(ctx, roots, uint64(i), eds)
		}
	})

	// read 128 EDSs does not read full EDS, but only the header
	// BenchmarkStore/open_by_height,_128-16         	 1585693	       747.6 ns/op (~7mcs)
	b.Run("open by height, 128", func(b *testing.B) {
		edsStore, err := NewStore(paramsNoCache(), b.TempDir())
		require.NoError(b, err)

		height := uint64(1984)
		err = edsStore.PutODSQ4(ctx, roots, height, eds)
		require.NoError(b, err)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			f, err := edsStore.GetByHeight(ctx, height)
			require.NoError(b, err)
			require.NoError(b, f.Close())
		}
	})

	// BenchmarkStore/open_by_hash,_128-16           	 1240942	       945.9 ns/op (~9mcs)
	b.Run("open by hash, 128", func(b *testing.B) {
		edsStore, err := NewStore(DefaultParameters(), b.TempDir())
		require.NoError(b, err)

		height := uint64(1984)
		err = edsStore.PutODSQ4(ctx, roots, height, eds)
		require.NoError(b, err)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			f, err := edsStore.GetByHash(ctx, roots.Hash())
			require.NoError(b, err)
			require.NoError(b, f.Close())
		}
	})
}

func randomEDS(t testing.TB) (*rsmt2d.ExtendedDataSquare, *share.AxisRoots) {
	eds := edstest.RandEDS(t, 4)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)

	return eds, roots
}

func ensureAmountFileAndLinks(t testing.TB, dir string, files, links int) {
	ensureAmountFiles(t, dir, files)
	ensureAmountLinks(t, dir, links)
}

func ensureAmountFiles(t testing.TB, dir string, files int) {
	// add empty file ods and q4 parts and heights folder to the count
	files += 3
	// ensure block folder contains the correct amount of files
	blockPath := path.Join(dir, blocksPath)
	entries, err := os.ReadDir(blockPath)
	require.NoError(t, err)
	require.Len(t, entries, files)
}

func ensureAmountLinks(t testing.TB, dir string, links int) {
	// ensure heights folder contains the correct amount of links
	linksPath := path.Join(dir, heightsPath)
	entries, err := os.ReadDir(linksPath)
	require.NoError(t, err)
	require.Len(t, entries, links)
}

func hasByHashAndHeight(
	t testing.TB,
	store *Store,
	ctx context.Context,
	hash share.DataHash,
	height uint64,
	hasByHash, hasByHeight bool,
) {
	has, err := store.HasByHash(ctx, hash)
	require.NoError(t, err)
	require.Equal(t, hasByHash, has)

	has, err = store.HasByHeight(ctx, height)
	require.NoError(t, err)
	require.Equal(t, hasByHeight, has)
}

type putFunc func(
	ctx context.Context,
	roots *share.AxisRoots,
	height uint64,
	square *rsmt2d.ExtendedDataSquare,
) error
