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

	// 15 headers from the past and 15 future headers
	mockGet, shareServ, sub := createDASerSubcomponents(t, 15, 15)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	daser := NewDASer(shareServ, sub, mockGet, ds)

	err := daser.Start(ctx)
	require.NoError(t, err)

	// wait for dasing catch-up routine to finish
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-mockGet.doneCh:
	}

	err = daser.Stop(ctx)
	require.NoError(t, err)

	// load checkpoint and ensure it's at network head
	checkpoint, err := loadCheckpoint(daser.cstore)
	require.NoError(t, err)
	// ensure checkpoint is stored at 15
	assert.Equal(t, int64(15), checkpoint)
}

func TestDASer_catchUp(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	cstore := wrapCheckpointStore(ds)

	mockGet, shareServ, _ := createDASerSubcomponents(t, 5, 0)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	daser := NewDASer(shareServ, nil, mockGet, cstore)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		// catch up from height 2 to head
		job := &catchUpJob{
			from: 2,
			to:   mockGet.head,
		}
		daser.catchUp(ctx, job)
		wg.Done()
	}(wg)
	wg.Wait()
}

// TestDASer_catchUp_oneHeader tests that catchUp works with a from-to
// difference of 1
func TestDASer_catchUp_oneHeader(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	cstore := wrapCheckpointStore(ds)

	mockGet, shareServ, _ := createDASerSubcomponents(t, 6, 0)

	// store checkpoint
	err := storeCheckpoint(cstore, 5) // pick arbitrary height as last checkpoint
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	daser := NewDASer(shareServ, nil, mockGet, cstore)
	checkpoint, err := loadCheckpoint(cstore)
	require.NoError(t, err)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		job := &catchUpJob{
			from: checkpoint,
			to:   mockGet.head,
		}
		daser.catchUp(ctx, job)
		wg.Done()
	}(wg)
	wg.Wait()
}

// createDASerSubcomponents takes numGetter (number of headers
// to store in mockGetter) and numSub (number of headers to store
// in the mock header.Subscriber), returning a newly instantiated
// mockGetter, share.Service, and mock header.Subscriber.
func createDASerSubcomponents(t *testing.T, numGetter, numSub int) (*mockGetter, share.Service, header.Subscriber) {
	dag := mdutils.Mock()
	shareServ := share.NewService(dag, share.NewLightAvailability(dag))

	mockGet := &mockGetter{
		headers: make(map[int64]*header.ExtendedHeader),
		doneCh:  make(chan struct{}),
	}

	// generate 15 headers from the past for HeaderGetter
	for i := 0; i < numGetter; i++ {
		dah := share.RandFillDAG(t, 16, dag)

		randHeader := header.RandExtendedHeader(t)
		randHeader.DataHash = dah.Hash()
		randHeader.DAH = dah
		randHeader.Height = int64(i + 1)

		mockGet.headers[int64(i+1)] = randHeader
	}
	mockGet.head = int64(numGetter) // network head

	sub := &header.DummySubscriber{
		Headers: make([]*header.ExtendedHeader, numSub),
	}

	// generate 15 headers from the future for p2pSub to pipe through to DASer
	index := 0
	for i := numGetter; i < numGetter+numSub; i++ {
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
	doneCh chan struct{} // signals all stored headers have been retrieved

	head    int64
	headers map[int64]*header.ExtendedHeader
}

func (m *mockGetter) Head(context.Context) (*header.ExtendedHeader, error) {
	return m.headers[m.head], nil
}

func (m *mockGetter) GetByHeight(_ context.Context, height uint64) (*header.ExtendedHeader, error) {
	defer func() {
		if int64(height) == m.head {
			close(m.doneCh)
		}
	}()
	return m.headers[int64(height)], nil
}
