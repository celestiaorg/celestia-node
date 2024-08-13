//go:build !race

package nodebuilder

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v2/pkg/da"
	"github.com/celestiaorg/celestia-app/v2/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestRepo(t *testing.T) {
	tests := []struct {
		tp node.Type
	}{
		{tp: node.Bridge}, {tp: node.Light}, {tp: node.Full},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			dir := t.TempDir()

			_, err := OpenStore(dir, nil)
			assert.ErrorIs(t, err, ErrNotInited)

			err = Init(*DefaultConfig(tt.tp), dir, tt.tp)
			require.NoError(t, err)

			store, err := OpenStore(dir, nil)
			require.NoError(t, err)

			_, err = OpenStore(dir, nil)
			assert.ErrorIs(t, err, ErrOpened)

			ks, err := store.Keystore()
			assert.NoError(t, err)
			assert.NotNil(t, ks)

			data, err := store.Datastore()
			assert.NoError(t, err)
			assert.NotNil(t, data)

			cfg, err := store.Config()
			assert.NoError(t, err)
			assert.NotNil(t, cfg)

			err = store.Close()
			assert.NoError(t, err)
		})
	}
}

func BenchmarkStore(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	b.Cleanup(cancel)

	// BenchmarkStore/bench_read_128-10         	      14	  78970661 ns/op (~70ms)
	b.Run("bench put 128", func(b *testing.B) {
		dir := b.TempDir()
		err := Init(*DefaultConfig(node.Full), dir, node.Full)
		require.NoError(b, err)

		store := newStore(ctx, b, eds.DefaultParameters(), dir)
		size := 128
		b.Run("enabled eds proof caching", func(b *testing.B) {
			b.StopTimer()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				adder := ipld.NewProofsAdder(size * 2)
				shares := sharetest.RandShares(b, size*size)
				eds, err := rsmt2d.ComputeExtendedDataSquare(
					shares,
					share.DefaultRSMT2DCodec(),
					wrapper.NewConstructor(uint64(size),
						nmt.NodeVisitor(adder.VisitFn())),
				)
				require.NoError(b, err)
				dah, err := da.NewDataAvailabilityHeader(eds)
				require.NoError(b, err)
				ctx := ipld.CtxWithProofsAdder(ctx, adder)

				b.StartTimer()
				err = store.edsStore.Put(ctx, dah.Hash(), eds)
				b.StopTimer()
				require.NoError(b, err)
			}
		})

		b.Run("disabled eds proof caching", func(b *testing.B) {
			b.ResetTimer()
			b.StopTimer()
			for i := 0; i < b.N; i++ {
				eds := edstest.RandEDS(b, size)
				dah, err := da.NewDataAvailabilityHeader(eds)
				require.NoError(b, err)

				b.StartTimer()
				err = store.edsStore.Put(ctx, dah.Hash(), eds)
				b.StopTimer()
				require.NoError(b, err)
			}
		})
	})
}

func TestStoreRestart(t *testing.T) {
	const (
		blocks = 5
		size   = 32
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	dir := t.TempDir()
	err := Init(*DefaultConfig(node.Full), dir, node.Full)
	require.NoError(t, err)

	store := newStore(ctx, t, eds.DefaultParameters(), dir)

	hashes := make([][]byte, blocks)
	for i := range hashes {
		edss := edstest.RandEDS(t, size)
		require.NoError(t, err)
		dah, err := da.NewDataAvailabilityHeader(edss)
		require.NoError(t, err)
		err = store.edsStore.Put(ctx, dah.Hash(), edss)
		require.NoError(t, err)

		// store hashes for read loop later
		hashes[i] = dah.Hash()
	}

	// restart store
	store.stop(ctx, t)
	store = newStore(ctx, t, eds.DefaultParameters(), dir)

	for _, h := range hashes {
		edsReader, err := store.edsStore.GetCAR(ctx, h)
		require.NoError(t, err)
		odsReader, err := eds.ODSReader(edsReader)
		require.NoError(t, err)
		_, err = eds.ReadEDS(ctx, odsReader, h)
		require.NoError(t, err)
		require.NoError(t, edsReader.Close())
	}
}

func TestDiscoverOpened(t *testing.T) {
	t.Run("single open store", func(t *testing.T) {
		_, dir := initAndOpenStore(t, node.Full)

		mockDefaultNodeStorePath := func(t, n string) (string, error) {
			return dir, nil
		}
		DefaultNodeStorePath = mockDefaultNodeStorePath

		path, err := DiscoverOpened()
		require.NoError(t, err)
		require.Equal(t, dir, path)
	})

	t.Run("multiple open nodes by preference order", func(t *testing.T) {
		networks := []p2p.Network{p2p.Mainnet, p2p.Mocha, p2p.Arabica, p2p.Private}
		nodeTypes := []node.Type{node.Bridge, node.Full, node.Light}

		// Store opened stores in a map (network + node -> dir/store)
		dirMap := make(map[string]string)
		storeMap := make(map[string]Store)
		for _, network := range networks {
			for _, tp := range nodeTypes {
				store, dir := initAndOpenStore(t, tp)
				key := network.String() + "_" + tp.String()
				dirMap[key] = dir
				storeMap[key] = store
			}
		}

		mockDefaultNodeStorePath := func(tp, n string) (string, error) {
			key := n + "_" + tp
			if dir, ok := dirMap[key]; ok {
				return dir, nil
			}
			return "", fmt.Errorf("no store for %s_%s", n, tp)
		}
		DefaultNodeStorePath = mockDefaultNodeStorePath

		// Discover opened stores in preference order
		for _, network := range networks {
			for _, tp := range nodeTypes {
				path, err := DiscoverOpened()
				require.NoError(t, err)
				key := network.String() + "_" + tp.String()
				require.Equal(t, dirMap[key], path)

				// close the store to discover the next one
				storeMap[key].Close()
			}
		}
	})

	t.Run("no opened store", func(t *testing.T) {
		dir := t.TempDir()
		mockDefaultNodeStorePath := func(t, n string) (string, error) {
			return dir, nil
		}
		DefaultNodeStorePath = mockDefaultNodeStorePath

		path, err := DiscoverOpened()
		assert.ErrorIs(t, err, ErrNoOpenStore)
		assert.Empty(t, path)
	})
}

func TestIsOpened(t *testing.T) {
	dir := t.TempDir()

	// Case 1: non-existent node store
	ok, err := IsOpened(dir)
	require.NoError(t, err)
	require.False(t, ok)

	// Case 2: initialized node store, not locked
	err = Init(*DefaultConfig(node.Full), dir, node.Full)
	require.NoError(t, err)
	ok, err = IsOpened(dir)
	require.NoError(t, err)
	require.False(t, ok)

	// Case 3: initialized node store, locked
	flk := flock.New(lockPath(dir))
	_, err = flk.TryLock()
	require.NoError(t, err)
	defer flk.Unlock() //nolint:errcheck
	ok, err = IsOpened(dir)
	require.NoError(t, err)
	require.True(t, ok)
}

func initAndOpenStore(t *testing.T, tp node.Type) (store Store, dir string) {
	dir = t.TempDir()
	err := Init(*DefaultConfig(tp), dir, tp)
	require.NoError(t, err)
	store, err = OpenStore(dir, nil)
	require.NoError(t, err)
	return store, dir
}

type store struct {
	s        Store
	edsStore *eds.Store
}

func newStore(ctx context.Context, t testing.TB, params *eds.Parameters, dir string) store {
	s, err := OpenStore(dir, nil)
	require.NoError(t, err)
	ds, err := s.Datastore()
	require.NoError(t, err)
	edsStore, err := eds.NewStore(params, dir, ds)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)
	return store{
		s:        s,
		edsStore: edsStore,
	}
}

func (s *store) stop(ctx context.Context, t *testing.T) {
	require.NoError(t, s.edsStore.Stop(ctx))
	require.NoError(t, s.s.Close())
}
