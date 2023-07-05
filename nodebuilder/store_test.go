package nodebuilder

import (
	"context"
	"strconv"
	"testing"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestRepo(t *testing.T) {
	var tests = []struct {
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

	tmpDir := b.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	edsStore, err := eds.NewStore(tmpDir, ds)
	require.NoError(b, err)
	err = edsStore.Start(ctx)
	require.NoError(b, err)

	// BenchmarkStore/bench_read_128-10         	      14	  78970661 ns/op (~70ms)
	b.Run("bench put 128", func(b *testing.B) {
		b.ResetTimer()
		dir := b.TempDir()

		err := Init(*DefaultConfig(node.Full), dir, node.Full)
		require.NoError(b, err)

		store, err := OpenStore(dir, nil)
		require.NoError(b, err)
		ds, err := store.Datastore()
		require.NoError(b, err)
		edsStore, err := eds.NewStore(dir, ds)
		require.NoError(b, err)
		err = edsStore.Start(ctx)
		require.NoError(b, err)

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
				dah := da.NewDataAvailabilityHeader(eds)

				b.StartTimer()
				err = edsStore.Put(ctx, dah.Hash(), eds)
				b.StopTimer()
				require.NoError(b, err)
			}
		})

		b.Run("disabled eds proof caching", func(b *testing.B) {
			b.ResetTimer()
			b.StopTimer()
			for i := 0; i < b.N; i++ {
				eds := edstest.RandEDS(b, size)
				dah := da.NewDataAvailabilityHeader(eds)

				b.StartTimer()
				err = edsStore.Put(ctx, dah.Hash(), eds)
				b.StopTimer()
				require.NoError(b, err)
			}
		})
	})
}
