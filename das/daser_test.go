package das

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	format "github.com/ipfs/go-ipld-format"
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
	cstore := wrapCheckpointStore(ds)
	dag := mdutils.Mock()

	// 15 headers from the past and 15 future headers
	mockGet, shareServ, sub := createDASerSubcomponents(t, dag, 15, 15)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	daser := NewDASer(shareServ, sub, mockGet, cstore)

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

func TestDASer_Restart(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	cstore := wrapCheckpointStore(ds)
	dag := mdutils.Mock()

	// 15 headers from the past and 15 future headers
	mockGet, shareServ, sub := createDASerSubcomponents(t, dag, 15, 15)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	daser := NewDASer(shareServ, sub, mockGet, cstore)

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

	// reset mockGet, generate 15 "past" headers, building off chain head which is 30
	mockGet.generateHeaders(t, dag, 30, 45)
	mockGet.doneCh = make(chan struct{})
	// reset dummy subscriber
	mockGet.fillSubWithHeaders(t, sub, dag, 45, 60)
	// manually set mockGet head to trigger stop at 45
	mockGet.head = int64(45)

	// restart DASer with new context
	restartCtx, restartCancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(restartCancel)

	err = daser.Start(restartCtx)
	require.NoError(t, err)

	// wait for dasing catch-up routine to finish
	select {
	case <-restartCtx.Done():
		t.Fatal(ctx.Err())
	case <-mockGet.doneCh:
	}

	err = daser.Stop(restartCtx)
	require.NoError(t, err)

	// load checkpoint and ensure it's at network head
	checkpoint, err := loadCheckpoint(daser.cstore)
	require.NoError(t, err)
	// ensure checkpoint is stored at 45
	assert.Equal(t, int64(45), checkpoint)
}

func TestDASer_catchUp(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	cstore := wrapCheckpointStore(ds)
	dag := mdutils.Mock()

	mockGet, shareServ, _ := createDASerSubcomponents(t, dag, 5, 0)

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
	dag := mdutils.Mock()

	mockGet, shareServ, _ := createDASerSubcomponents(t, dag, 6, 0)

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
func createDASerSubcomponents(
	t *testing.T,
	dag format.DAGService,
	numGetter,
	numSub int,
) (*mockGetter, share.Service, *header.DummySubscriber) {
	shareServ := share.NewService(dag, share.NewLightAvailability(dag))

	mockGet := &mockGetter{
		headers: make(map[int64]*header.ExtendedHeader),
		doneCh:  make(chan struct{}),
	}

	mockGet.generateHeaders(t, dag, 0, numGetter)

	sub := new(header.DummySubscriber)
	mockGet.fillSubWithHeaders(t, sub, dag, numGetter, numGetter+numSub)

	return mockGet, shareServ, sub
}

// fillSubWithHeaders generates `num` headers from the future for p2pSub to pipe through to DASer.
func (m *mockGetter) fillSubWithHeaders(
	t *testing.T,
	sub *header.DummySubscriber,
	dag format.DAGService,
	startHeight,
	endHeight int,
) {
	sub.Headers = make([]*header.ExtendedHeader, endHeight-startHeight)

	index := 0
	for i := startHeight; i < endHeight; i++ {
		dah := share.RandFillDAG(t, 16, dag)

		randHeader := header.RandExtendedHeader(t)
		randHeader.DataHash = dah.Hash()
		randHeader.DAH = dah
		randHeader.Height = int64(i + 1)

		sub.Headers[index] = randHeader
		// also store to mock getter for duplicate fetching
		m.headers[int64(i+1)] = randHeader

		index++
	}
}

type mockGetter struct {
	doneCh chan struct{} // signals all stored headers have been retrieved

	head    int64
	headers map[int64]*header.ExtendedHeader
}

func (m *mockGetter) generateHeaders(t *testing.T, dag format.DAGService, startHeight, endHeight int) {
	for i := startHeight; i < endHeight; i++ {
		dah := share.RandFillDAG(t, 16, dag)

		randHeader := header.RandExtendedHeader(t)
		randHeader.DataHash = dah.Hash()
		randHeader.DAH = dah
		randHeader.Height = int64(i + 1)

		m.headers[int64(i+1)] = randHeader
	}
	// set network head
	m.head = int64(startHeight + endHeight)
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
