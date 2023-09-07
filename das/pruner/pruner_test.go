package pruner

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	ds_sync "github.com/ipfs/go-datastore/sync"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/mocks"
	"github.com/celestiaorg/celestia-node/share/p2p/discovery"
	"github.com/celestiaorg/rsmt2d"
	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/require"
)

func TestStoragePruner_Prunes(t *testing.T) {
	var (
		count         = 20
		recencyWindow = 5
	)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ctrl := gomock.NewController(t)
	getter := mocks.NewMockGetter(ctrl)
	tmpDir := t.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	edsStore, err := eds.NewStore(tmpDir, ds)
	require.NoError(t, err)
	availability := testAvailability(edsStore, getter)

	// Create a new StoragePruner
	sp, err := NewStoragePruner(edsStore, ds, getter, availability, &Config{
		RecencyWindow: uint64(recencyWindow),
	})
	require.NoError(t, err)

	err = edsStore.Start(ctx)
	require.NoError(t, err)
	err = sp.Start(ctx)
	require.NoError(t, err)

	// Setup: Generate count test EDSes and make the mock getter expect them
	edses := make([]*rsmt2d.ExtendedDataSquare, count)
	extendedHeaders := make([]*header.ExtendedHeader, count)
	for i := 0; i < count; i++ {
		eds := edstest.RandEDS(t, 16)
		edses[i] = eds
		dah, err := da.NewDataAvailabilityHeader(eds)

		randHeader := headertest.RandExtendedHeader(t)
		randHeader.DataHash = dah.Hash()
		randHeader.DAH = &dah
		randHeader.RawHeader.Height = int64(i + 1)
		extendedHeaders[i] = randHeader

		require.NoError(t, err)

		getter.EXPECT().GetEDS(gomock.Any(), &dah).Return(eds, nil).AnyTimes()
	}

	// 1. Verify that the first EDSes get stored and are retrievable.
	for i := 0; i < recencyWindow; i++ {
		sp.SampleAndRegister(ctx, extendedHeaders[i])
	}

	for i := 0; i < recencyWindow; i++ {
		has, err := edsStore.Has(ctx, extendedHeaders[i].DAH.Hash())
		require.True(t, has)
		require.NoError(t, err)
	}

	// 2. Verify that after RecencyWindow EDSes, the oldest EDSes get pruned.
	for i := recencyWindow; i < count; i++ {
		sp.SampleAndRegister(ctx, extendedHeaders[i])
		// It takes time for the old data to be pruned, so check 10 times
		for j := 0; j < 10; j++ {
			time.Sleep(100 * time.Millisecond)
			has, err := edsStore.Has(ctx, extendedHeaders[i-recencyWindow].DAH.Hash())
			require.False(t, has)
			require.NoError(t, err)
		}
	}
}

func testAvailability(store *eds.Store, getter share.Getter) *full.ShareAvailability {
	disc := discovery.NewDiscovery(
		nil,
		routing.NewRoutingDiscovery(routinghelpers.Null{}),
		discovery.WithAdvertiseInterval(time.Second),
		discovery.WithPeersLimit(10),
	)
	return full.NewShareAvailability(store, getter, disc)
}
