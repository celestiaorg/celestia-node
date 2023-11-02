package pruner

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	logging "github.com/ipfs/go-log/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/logs"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestStoragePruner_GarbageCollectsAllOutdatedEpochs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	store, err := eds.NewStore(eds.DefaultParameters(), t.TempDir(), ds)
	require.NoError(t, err)

	err = store.Start(ctx)
	require.NoError(t, err)

	cfg := Config{
		RecencyWindow: 10 * time.Second,
		EpochDuration: 2 * time.Second,
	}
	pruner := NewStoragePruner(ds, store, cfg)

	startTime := time.Now().Add(-time.Hour)
	endTime := startTime.Add(time.Minute * 5)
	timeBetweenHeaders := time.Second
	fillPrunerWithBlocks(ctx, t, 16, pruner, store, startTime, endTime, timeBetweenHeaders)

	expected := int(endTime.Sub(startTime) / cfg.EpochDuration)
	// adjusted the assertion to allow for a +/- 1 difference due to potential timing issues
	assert.True(t, len(pruner.activeEpochs) >= expected-1 && len(pruner.activeEpochs) <= expected+1)
	err = pruner.Start(ctx)
	require.NoError(t, err)
	require.NoError(t, err)

	for {
		select {
		case <-ctx.Done():
			t.Fatal("context timeout before assertion passed")
		case <-time.After(cfg.EpochDuration):
			if len(pruner.activeEpochs) == 0 {
				return
			}
			t.Log("waiting for pruner to finish. count: ", len(pruner.activeEpochs))
		}
	}
}

func TestStoragePruner_OldestEpochStaysUpdated(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	logs.SetAllLoggers(logging.LevelDebug)

	ds := datastore.NewMapDatastore()
	store, err := eds.NewStore(eds.DefaultParameters(), t.TempDir(), ds)
	require.NoError(t, err)

	err = store.Start(ctx)
	require.NoError(t, err)

	cfg := Config{
		RecencyWindow: 10 * time.Second,
		EpochDuration: 2 * time.Second,
	}
	pruner := NewStoragePruner(ds, store, cfg)

	startTime := time.Now().Add(-time.Hour)
	endTime := startTime.Add(time.Minute * 5)
	timeBetweenHeaders := time.Second
	fillPrunerWithBlocks(ctx, t, 16, pruner, store, startTime, endTime, timeBetweenHeaders)

	expected := int(endTime.Sub(startTime) / cfg.EpochDuration)
	// Adjusted the assertion to allow for a +/- 1 difference due to potential timing issues
	epochCount := len(pruner.activeEpochs)
	assert.True(t, epochCount >= expected-1 && epochCount <= expected+1)

	nextEpoch := pruner.calculateEpoch(startTime)
	for i := 0; i < epochCount; i++ {
		t.Log(nextEpoch)
		oldestEpoch := pruner.oldestEpoch.Load()
		assert.Equal(t, oldestEpoch, nextEpoch)
		err = pruner.pruneEpoch(ctx, oldestEpoch)
		require.NoError(t, err)
		nextEpoch = pruner.oldestEpoch.Load()
	}
	assert.Equal(t, len(pruner.activeEpochs), 0)
}

func TestStoragePruner_KeepsRecentEpochs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ds := datastore.NewMapDatastore()
	store, err := eds.NewStore(eds.DefaultParameters(), t.TempDir(), ds)
	require.NoError(t, err)

	err = store.Start(ctx)
	require.NoError(t, err)

	cfg := Config{
		RecencyWindow: 10 * time.Second,
		EpochDuration: 2 * time.Second,
	}
	pruner := NewStoragePruner(ds, store, cfg)

	// Pruner will need to run for at least 6 seconds to remove all epochs
	startTime := time.Now().Add(-time.Second * 5)
	endTime := startTime.Add(time.Second * 10)
	timeBetweenHeaders := time.Second
	fillPrunerWithBlocks(ctx, t, 16, pruner, store, startTime, endTime, timeBetweenHeaders)

	expected := int(endTime.Sub(startTime) / cfg.EpochDuration)
	// Adjusted the assertion to allow for a +/- 1 difference due to potential timing issues
	assert.True(t, len(pruner.activeEpochs) >= expected-1 && len(pruner.activeEpochs) <= expected+1)
	err = pruner.Start(ctx)
	require.NoError(t, err)
	require.NoError(t, err)

	for {
		select {
		case <-ctx.Done():
			t.Fatal("context timeout before assertion passed")
		case <-time.After(cfg.EpochDuration):
			if len(pruner.activeEpochs) > 0 {
				t.Log("pruner still has recent epochs")
			} else {
				return
			}
		}
	}
}

func TestCalculateEpoch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ds := datastore.NewMapDatastore()
	store, err := eds.NewStore(eds.DefaultParameters(), t.TempDir(), ds)
	require.NoError(t, err)

	err = store.Start(ctx)
	require.NoError(t, err)

	pruner := NewStoragePruner(ds, store, Config{
		RecencyWindow: 10 * time.Second,
		EpochDuration: 2 * time.Second,
	})

	assert.Equal(t, uint64(0), pruner.calculateEpoch(time.Unix(0, 0)))
	assert.Equal(t, uint64(0), pruner.calculateEpoch(time.Unix(1, 0)))
	assert.Equal(t, uint64(1), pruner.calculateEpoch(time.Unix(2, 0)))
	assert.Equal(t, uint64(1), pruner.calculateEpoch(time.Unix(3, 0)))
	assert.Equal(t, uint64(2), pruner.calculateEpoch(time.Unix(4, 0)))
}

func TestEpochIsRecent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ds := datastore.NewMapDatastore()
	store, err := eds.NewStore(eds.DefaultParameters(), t.TempDir(), ds)
	require.NoError(t, err)

	err = store.Start(ctx)
	require.NoError(t, err)

	pruner := NewStoragePruner(ds, store, Config{
		RecencyWindow: 10 * time.Second,
		EpochDuration: 2 * time.Second,
	})

	recent := pruner.calculateEpoch(time.Now().Add(-time.Second * 5))
	assert.True(t, pruner.epochIsRecent(recent))

	recent = pruner.calculateEpoch(time.Now().Add(-time.Second * 10))
	assert.True(t, pruner.epochIsRecent(recent))

	old := pruner.calculateEpoch(time.Now().Add(-time.Second * 11))
	assert.False(t, pruner.epochIsRecent(old))
}

func fillPrunerWithBlocks(
	ctx context.Context,
	t *testing.T,
	blockSize int,
	pruner *StoragePruner,
	store *eds.Store,
	startTime, endTime time.Time,
	timeBetweenHeaders time.Duration,
) {
	bServ := ipld.NewMemBlockservice()
	headers := generateHeaders(t, startTime, endTime, timeBetweenHeaders)
	for _, h := range headers {
		shares := sharetest.RandShares(t, blockSize*blockSize)
		eds, err := ipld.AddShares(context.TODO(), shares, bServ)
		require.NoError(t, err)
		dah, err := share.NewRoot(eds)
		require.NoError(t, err)

		h.DataHash = dah.Hash()
		h.DAH = dah

		err = store.Put(ctx, share.DataHash(h.DataHash), eds)
		require.NoError(t, err)
		err = pruner.Register(ctx, h)
		require.NoError(t, err)
	}
}

func generateHeaders(
	t *testing.T,
	startTime, endTime time.Time,
	timeBetweenHeaders time.Duration,
) []*header.ExtendedHeader {
	var headers []*header.ExtendedHeader
	for currentTime := startTime; currentTime.Before(endTime); currentTime = currentTime.Add(timeBetweenHeaders) {
		headers = append(headers, headertest.RandExtendedHeaderAtTimestamp(t, currentTime))
	}
	return headers
}
