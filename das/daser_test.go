package das

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

// TestDASerLifecycle tests to ensure every mock block is DASed and
// the DASer checkpoint is updated to network head.
func TestDASerLifecycle(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	cstore := NewCheckpointStore(ds) // we aren't storing the checkpoint so that DASer starts DASing from height 1.

	mockGet, shareServ, sub := createDASerSubcomponents(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	daser := NewDASer(shareServ, sub, mockGet, cstore)

	err := daser.Start(ctx)
	require.NoError(t, err)

	select {
	// wait for sampleLatest routine to finish so that it stores the
	// latest DASed checkpoint so that it stores the latest DASed checkpoint
	case <-daser.sampleLatestDn:
	case <-ctx.Done():
	}

	// load checkpoint and ensure it's at network head
	checkpoint, err := loadCheckpoint(cstore)
	require.NoError(t, err)
	assert.Equal(t, int64(30), checkpoint)

	err = daser.Stop(ctx)
	require.NoError(t, err)
}

func TestDASer_sampleLatest(t *testing.T) {
	shareServ, dah := share.RandLightServiceWithSquare(t, 16)

	randHeader := header.RandExtendedHeader(t)
	randHeader.DataHash = dah.Hash()
	randHeader.DAH = dah

	sub := &header.DummySubscriber{
		Headers: []*header.ExtendedHeader{randHeader},
	}

	mockGet := new(mockGetter)
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	cstore := NewCheckpointStore(ds)

	daser := NewDASer(shareServ, sub, mockGet, cstore)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		daser.sampleLatest(context.Background(), sub, 0)
		wg.Done()
	}(wg)
	wg.Wait()
}

func TestDASer_sampleCheckpoint(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	cstore := NewCheckpointStore(ds)

	mockGet, shareServ, _ := createDASerSubcomponents(t)

	// store checkpoint
	err := storeCheckpoint(cstore, 2) // pick random header as last checkpoint
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	daser := NewDASer(shareServ, nil, mockGet, cstore)
	checkpoint, err := loadCheckpoint(cstore)
	require.NoError(t, err)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		daser.sampleFromCheckpoint(ctx, checkpoint, mockGet.head)
		wg.Done()
	}(wg)
	wg.Wait()
}

func createDASerSubcomponents(t *testing.T) (*mockGetter, share.Service, header.Subscriber) {
	dag := mdutils.Mock()
	shareServ := share.NewService(dag, share.NewLightAvailability(dag))

	mockGet := &mockGetter{
		headers: make(map[int64]*header.ExtendedHeader),
	}

	// generate 15 headers from the past for HeaderGetter
	for i := 0; i < 15; i++ {
		dah := share.RandFillDAG(t, 16, dag)

		randHeader := header.RandExtendedHeader(t)
		randHeader.DataHash = dah.Hash()
		randHeader.DAH = dah
		randHeader.Height = int64(i + 1)

		mockGet.headers[int64(i+1)] = randHeader
	}
	mockGet.head = 15 // network head

	sub := &header.DummySubscriber{
		Headers: make([]*header.ExtendedHeader, 15),
	}

	// generate 15 headers from the future for p2pSub to pipe through to DASer
	index := 0
	for i := 15; i < 30; i++ {
		dah := share.RandFillDAG(t, 16, dag)

		randHeader := header.RandExtendedHeader(t)
		randHeader.DataHash = dah.Hash()
		randHeader.DAH = dah
		randHeader.Height = int64(i + 1)

		sub.Headers[index] = randHeader
		index++
	}

	return mockGet, shareServ, sub
}

type mockGetter struct {
	head    int64
	headers map[int64]*header.ExtendedHeader
}

func (m mockGetter) Head(context.Context) (*header.ExtendedHeader, error) {
	return m.headers[m.head], nil
}

func (m mockGetter) GetByHeight(_ context.Context, height uint64) (*header.ExtendedHeader, error) {
	return m.headers[int64(height)], nil
}
