package eds

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/dagstore/shard"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/ipld/go-car"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/da"

	"github.com/celestiaorg/celestia-node/share"

	"github.com/celestiaorg/rsmt2d"
)

// TestEDSStore_PutRegistersShard tests if Put registers the shard on the underlying DAGStore
func TestEDSStore_PutRegistersShard(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newEDSStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds, dah := randomEDS(t)

	// shard hasn't been registered yet
	_, err = edsStore.dgstr.GetShardInfo(shard.KeyFromString(dah.String()))
	require.Error(t, err, "shard not found")

	err = edsStore.Put(ctx, dah, eds)
	require.NoError(t, err)

	_, err = edsStore.dgstr.GetShardInfo(shard.KeyFromString(dah.String()))
	require.NoError(t, err)
}

// TestEDSStore_PutIndexesEDS ensures that Putting an EDS indexes it into the car index
func TestEDSStore_PutIndexesEDS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newEDSStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds, dah := randomEDS(t)
	stat, _ := edsStore.carIdx.StatFullIndex(shard.KeyFromString(dah.String()))
	require.False(t, stat.Exists)

	err = edsStore.Put(ctx, dah, eds)
	require.NoError(t, err)

	stat, err = edsStore.carIdx.StatFullIndex(shard.KeyFromString(dah.String()))
	require.True(t, stat.Exists)
	require.NoError(t, err)
}

// TestEDSStore_GetCAR ensures that the reader returned from GetCAR is capable of reading the CAR header and ODS.
// NOTE: I haven't been able to come up with a reasonable way to restrict the reader to only read the ODS.
// This part of the design needs to be discussed.
func TestEDSStore_GetCAR(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newEDSStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds, dah := randomEDS(t)
	err = edsStore.Put(ctx, dah, eds)
	require.NoError(t, err)

	r, err := edsStore.GetCAR(ctx, dah)
	require.NoError(t, err)
	carReader, err := car.NewCarReader(r)

	fmt.Println(car.HeaderSize(carReader.Header))
	require.NoError(t, err)

	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			original := eds.GetCell(uint(i), uint(j))
			block, err := carReader.Next()
			require.NoError(t, err)
			require.Equal(t, original, block.RawData()[share.NamespaceSize:])
		}
	}
}

func TestEDSStore_GetCARRestrictsToODS(t *testing.T) {
	t.Skip()
}

func TestEDSStore_ContainsEmptyRoot(t *testing.T) {
	t.Skip()
}

func TestEDSStore_Remove(t *testing.T) {
	// Should also not be in blockstore? There is a cache there
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newEDSStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds, dah := randomEDS(t)

	err = edsStore.Put(ctx, dah, eds)
	require.NoError(t, err)

	err = edsStore.Remove(ctx, dah)
	require.NoError(t, err)

	// shard should no longer be registered on the dagstore
	_, err = edsStore.dgstr.GetShardInfo(shard.KeyFromString(dah.String()))
	require.Error(t, err, "shard not found")

	// shard should have been dropped from the index, which also removes the file
	stat, err := edsStore.carIdx.StatFullIndex(shard.KeyFromString(dah.String()))
	require.NoError(t, err)
	require.False(t, stat.Exists)
}

func TestEDSStore_Has(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newEDSStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds, dah := randomEDS(t)

	ok, err := edsStore.Has(ctx, dah)
	require.Error(t, err, "shard not found")
	require.False(t, ok)

	err = edsStore.Put(ctx, dah, eds)
	require.NoError(t, err)

	ok, err = edsStore.Has(ctx, dah)
	require.NoError(t, err)
	require.True(t, ok)
}

func newEDSStore(t *testing.T) (*EDSStore, error) {
	t.Helper()

	tmpDir := t.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	edsStore, err := NewEDSStore(tmpDir, ds)

	return edsStore, err
}

func randomEDS(t *testing.T) (*rsmt2d.ExtendedDataSquare, share.Root) {
	eds := share.RandEDS(t, 4)
	dah := da.NewDataAvailabilityHeader(eds)

	return eds, dah
}
