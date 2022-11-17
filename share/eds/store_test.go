package eds

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/dagstore/shard"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/ipld/go-car"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/rsmt2d"
)

// TestEDSStore_PutRegistersShard tests if Put registers the shard on the underlying DAGStore
func TestEDSStore_PutRegistersShard(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds, dah := randomEDS(t)

	// shard hasn't been registered yet
	has, err := edsStore.Has(ctx, dah)
	assert.False(t, has)
	assert.Error(t, err, "shard not found")

	err = edsStore.Put(ctx, dah, eds)
	assert.NoError(t, err)

	_, err = edsStore.dgstr.GetShardInfo(shard.KeyFromString(dah.String()))
	assert.NoError(t, err)
}

// TestEDSStore_PutIndexesEDS ensures that Putting an EDS indexes it into the car index
func TestEDSStore_PutIndexesEDS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds, dah := randomEDS(t)
	stat, _ := edsStore.carIdx.StatFullIndex(shard.KeyFromString(dah.String()))
	assert.False(t, stat.Exists)

	err = edsStore.Put(ctx, dah, eds)
	assert.NoError(t, err)

	stat, err = edsStore.carIdx.StatFullIndex(shard.KeyFromString(dah.String()))
	assert.True(t, stat.Exists)
	assert.NoError(t, err)
}

// TestEDSStore_GetCAR ensures that the reader returned from GetCAR is capable of reading the CAR
// header and ODS.
func TestEDSStore_GetCAR(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds, dah := randomEDS(t)
	err = edsStore.Put(ctx, dah, eds)
	require.NoError(t, err)

	r, err := edsStore.GetCAR(ctx, dah)
	assert.NoError(t, err)
	carReader, err := car.NewCarReader(r)

	fmt.Println(car.HeaderSize(carReader.Header))
	assert.NoError(t, err)

	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			original := eds.GetCell(uint(i), uint(j))
			block, err := carReader.Next()
			assert.NoError(t, err)
			assert.Equal(t, original, block.RawData()[share.NamespaceSize:])
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
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds, dah := randomEDS(t)

	err = edsStore.Put(ctx, dah, eds)
	require.NoError(t, err)

	err = edsStore.Remove(ctx, dah)
	assert.NoError(t, err)

	// shard should no longer be registered on the dagstore
	_, err = edsStore.dgstr.GetShardInfo(shard.KeyFromString(dah.String()))
	assert.Error(t, err, "shard not found")

	// shard should have been dropped from the index, which also removes the file
	stat, err := edsStore.carIdx.StatFullIndex(shard.KeyFromString(dah.String()))
	assert.NoError(t, err)
	assert.False(t, stat.Exists)
}

func TestEDSStore_Has(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds, dah := randomEDS(t)

	ok, err := edsStore.Has(ctx, dah)
	assert.Error(t, err, "shard not found")
	assert.False(t, ok)

	err = edsStore.Put(ctx, dah, eds)
	assert.NoError(t, err)

	ok, err = edsStore.Has(ctx, dah)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func newStore(t *testing.T) (*Store, error) {
	t.Helper()

	tmpDir := t.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	return NewStore(tmpDir, ds)
}

func randomEDS(t *testing.T) (*rsmt2d.ExtendedDataSquare, share.Root) {
	eds := share.RandEDS(t, 4)
	dah := da.NewDataAvailabilityHeader(eds)

	return eds, dah
}
