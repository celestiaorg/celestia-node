package das

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

var timeout = time.Second * 15

// TestDASerLifecycle tests to ensure every mock block is DASed and
// the DASer checkpoint is updated to network head.
func TestDASerLifecycle(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	bServ := mdutils.Bserv()
	avail := share.TestLightAvailability(bServ)
	// 15 headers from the past and 15 future headers
	mockGet, shareServ, sub, mockService := createDASerSubcomponents(t, bServ, 15, 15, avail)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	daser := NewDASer(shareServ, sub, mockGet, ds, mockService)

	err := daser.Start(ctx)
	require.NoError(t, err)
	defer func() {
		err = daser.Stop(ctx)
		require.NoError(t, err)

		// load checkpoint and ensure it's at network head
		checkpoint, err := loadCheckpoint(ctx, daser.cstore)
		require.NoError(t, err)
		// ensure checkpoint is stored at 15
		assert.Equal(t, int64(15), checkpoint)
	}()
	// wait for dasing catch-up routine to finish
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-mockGet.doneCh:
	}
	// give catch-up routine a second to finish up sampling last header
	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		default:
			if daser.CatchUpRoutineState().Finished() {
				return
			}
		}
	}
}

func TestDASer_Restart(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	bServ := mdutils.Bserv()
	avail := share.TestLightAvailability(bServ)
	// 15 headers from the past and 15 future headers
	mockGet, shareServ, sub, mockService := createDASerSubcomponents(t, bServ, 15, 15, avail)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	daser := NewDASer(shareServ, sub, mockGet, ds, mockService)

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
	mockGet.generateHeaders(t, bServ, 30, 45)
	mockGet.doneCh = make(chan struct{})
	// reset dummy subscriber
	mockGet.fillSubWithHeaders(t, sub, bServ, 45, 60)
	// manually set mockGet head to trigger stop at 45
	mockGet.head = int64(45)

	// restart DASer with new context
	restartCtx, restartCancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(restartCancel)

	err = daser.Start(restartCtx)
	require.NoError(t, err)

	// wait for dasing catch-up routine to finish
	select {
	case <-restartCtx.Done():
		t.Fatal(restartCtx.Err())
	case <-mockGet.doneCh:
	}

	err = daser.Stop(restartCtx)
	require.NoError(t, err)

	assert.True(t, daser.CatchUpRoutineState().Finished())

	// load checkpoint and ensure it's at network head
	checkpoint, err := loadCheckpoint(ctx, daser.cstore)
	require.NoError(t, err)
	// ensure checkpoint is stored at 45
	assert.Equal(t, int64(45), checkpoint)
}

func TestDASer_catchUp(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	bServ := mdutils.Bserv()
	avail := share.TestLightAvailability(bServ)
	mockGet, shareServ, _, mockService := createDASerSubcomponents(t, bServ, 5, 0, avail)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	daser := NewDASer(shareServ, nil, mockGet, ds, mockService)

	type catchUpResult struct {
		checkpoint int64
		err        error
	}
	resultCh := make(chan *catchUpResult, 1)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		// catch up from height 2 to head
		job := &catchUpJob{
			from: 2,
			to:   mockGet.head,
		}
		checkpt, err := daser.catchUp(ctx, job)
		resultCh <- &catchUpResult{
			checkpoint: checkpt,
			err:        err,
		}
	}()
	wg.Wait()

	result := <-resultCh
	assert.Equal(t, mockGet.head, result.checkpoint)
	require.NoError(t, result.err)
}

// TestDASer_catchUp_oneHeader tests that catchUp works with a from-to
// difference of 1
func TestDASer_catchUp_oneHeader(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	bServ := mdutils.Bserv()
	avail := share.TestLightAvailability(bServ)
	mockGet, shareServ, _, mockService := createDASerSubcomponents(t, bServ, 6, 0, avail)
	daser := NewDASer(shareServ, nil, mockGet, ds, mockService)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// store checkpoint
	err := storeCheckpoint(ctx, daser.cstore, 5) // pick arbitrary height as last checkpoint
	require.NoError(t, err)

	checkpoint, err := loadCheckpoint(ctx, daser.cstore)
	require.NoError(t, err)

	type catchUpResult struct {
		checkpoint int64
		err        error
	}
	resultCh := make(chan *catchUpResult, 1)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		job := &catchUpJob{
			from: checkpoint,
			to:   mockGet.head,
		}
		checkpt, err := daser.catchUp(ctx, job)
		resultCh <- &catchUpResult{
			checkpoint: checkpt,
			err:        err,
		}
	}()
	wg.Wait()

	result := <-resultCh
	assert.Equal(t, mockGet.head, result.checkpoint)
	require.NoError(t, result.err)
}

func TestDASer_catchUp_fails(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	bServ := mdutils.Bserv()
	avail := share.TestLightAvailability(bServ)
	mockGet, _, _, mockService := createDASerSubcomponents(t, bServ, 6, 0, avail)
	daser := NewDASer(share.NewTestBrokenAvailability(), nil, mockGet, ds, mockService)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// store checkpoint
	err := storeCheckpoint(ctx, daser.cstore, 5) // pick arbitrary height as last checkpoint
	require.NoError(t, err)

	checkpoint, err := loadCheckpoint(ctx, daser.cstore)
	require.NoError(t, err)

	type catchUpResult struct {
		checkpoint int64
		err        error
	}
	resultCh := make(chan *catchUpResult, 1)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		job := &catchUpJob{
			from: checkpoint,
			to:   mockGet.head,
		}
		checkpt, err := daser.catchUp(ctx, job)
		resultCh <- &catchUpResult{
			checkpoint: checkpt,
			err:        err,
		}
	}()
	wg.Wait()

	result := <-resultCh
	require.ErrorIs(t, result.err, share.ErrNotAvailable)
}

func TestDASer_stopsAfter_BEFP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	bServ := mdutils.Bserv()
	// create mock network
	net, err := mocknet.FullMeshLinked(1)
	require.NoError(t, err)
	// create pubsub for host
	ps, err := pubsub.NewGossipSub(ctx, net.Hosts()[0],
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)
	avail := share.TestFullAvailability(bServ)
	// 15 headers from the past and 15 future headers
	mockGet, shareServ, sub, _ := createDASerSubcomponents(t, bServ, 15, 15, avail)

	// create fraud service and break one header
	f := fraud.NewService(ps, net.Hosts()[0], mockGet.GetByHeight, ds)
	mockGet.headers[1] = header.CreateFraudExtHeader(t, mockGet.headers[1], bServ)
	newCtx := context.Background()

	// create and start DASer
	daser := NewDASer(shareServ, sub, mockGet, ds, f)
	resultCh := make(chan error)
	go fraud.OnProof(newCtx, f, fraud.BadEncoding,
		func(fraud.Proof) {
			resultCh <- daser.Stop(ctx)
		})

	require.NoError(t, daser.Start(newCtx))
	// wait for fraud proof will be handled
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case res := <-resultCh:
		require.NoError(t, res)
	}
	require.True(t, daser.ctx.Err() == context.Canceled)
}

func TestDASerState(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	bServ := mdutils.Bserv()

	// 30 headers in the past and 30 headers in the future (so final sample height should be 60)
	mockGet, shareServ, sub, fraud := createDASerSubcomponents(t, bServ, 30, 30, share.NewTestSuccessfulAvailability())
	expectedFinalSampleHeight := uint64(60)
	expectedFinalSampleWidth := uint64(len(sub.Headers[29].DAH.RowsRoots))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	daser := NewDASer(shareServ, sub, mockGet, ds, fraud)
	err := daser.Start(ctx)
	require.NoError(t, err)
	defer func() {
		// wait for all "future" headers to be sampled
		for {
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			default:
				if daser.SampleRoutineState().LatestSampledHeight == expectedFinalSampleHeight {
					assert.Equal(t, expectedFinalSampleWidth, daser.SampleRoutineState().LatestSampledSquareWidth)

					err := daser.Stop(ctx)
					require.NoError(t, err)
					assert.False(t, daser.SampleRoutineState().IsRunning)
					return
				}
			}
		}
	}()
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-mockGet.doneCh:
	}
	// give catchUp routine a second to exit
	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		default:
			if daser.CatchUpRoutineState().Finished() {
				return
			}
		}
	}
}

// TestDASerState_WithErrorInCatchUp tests for the case where an
// error has occurred inside the catchUp routine, ensuring the catchUp
// routine exits as expected and reports the error accurately.
func TestDASerState_WithErrorInCatchUp(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	bServ := mdutils.Bserv()

	// 30 headers in the past and 30 headers in the future
	// catchUp routine error should occur at height 16
	brokenHeight := 16
	mockGet, shareServ, sub, fraud := createDASerWithFailingAvailability(t, bServ, 30, 30,
		int64(brokenHeight))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	daser := NewDASer(shareServ, sub, mockGet, ds, fraud)
	// allow catchUpDn signal to be read twice
	daser.catchUpDn = make(chan struct{}, 2)

	err := daser.Start(ctx)
	require.NoError(t, err)
	defer func() {
		err = daser.Stop(ctx)
		require.NoError(t, err)
	}()
	// wait until catchUp routine has fetched the broken header
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-mockGet.brokenHeightCh:
	}
	// wait for daser to exit catchUp routine
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-daser.catchUpDn:
		catchUp := daser.CatchUpRoutineState()
		// make sure catchUp routine didn't successfully complete
		assert.False(t, catchUp.Finished())
		// ensure that the error has been reported
		assert.NotNil(t, catchUp.Error)
		assert.Equal(t, uint64(brokenHeight)-1, catchUp.Height)
		// send another done signal so `Stop` can read it
		daser.catchUpDn <- struct{}{}
		return
	}
}

// createDASerSubcomponents takes numGetter (number of headers
// to store in mockGetter) and numSub (number of headers to store
// in the mock header.Subscriber), returning a newly instantiated
// mockGetter, share.Service, and mock header.Subscriber.
func createDASerSubcomponents(
	t *testing.T,
	bServ blockservice.BlockService,
	numGetter,
	numSub int,
	availability share.Availability,
) (*mockGetter, *share.Service, *header.DummySubscriber, *fraud.DummyService) {
	shareServ := share.NewService(bServ, availability)
	mockGet, sub := createMockGetterAndSub(t, bServ, numGetter, numSub)
	fraud := new(fraud.DummyService)
	return mockGet, shareServ, sub, fraud
}

func createMockGetterAndSub(
	t *testing.T,
	bServ blockservice.BlockService,
	numGetter,
	numSub int,
) (*mockGetter, *header.DummySubscriber) {
	mockGet := &mockGetter{
		headers:        make(map[int64]*header.ExtendedHeader),
		doneCh:         make(chan struct{}),
		brokenHeightCh: make(chan struct{}),
	}

	mockGet.generateHeaders(t, bServ, 0, numGetter)

	sub := new(header.DummySubscriber)
	mockGet.fillSubWithHeaders(t, sub, bServ, numGetter, numGetter+numSub)

	return mockGet, sub
}

func createDASerWithFailingAvailability(
	t *testing.T,
	bServ blockservice.BlockService,
	numGetter,
	numSub int,
	brokenHeight int64,
) (*mockGetter, *share.Service, *header.DummySubscriber, *fraud.DummyService) {
	mockGet, sub := createMockGetterAndSub(t, bServ, numGetter, numSub)
	mockGet.brokenHeight = brokenHeight

	shareServ := share.NewService(bServ, &share.TestBrokenAvailability{
		Root: mockGet.headers[brokenHeight].DAH,
	})

	return mockGet, shareServ, sub, new(fraud.DummyService)
}

// fillSubWithHeaders generates `num` headers from the future for p2pSub to pipe through to DASer.
func (m *mockGetter) fillSubWithHeaders(
	t *testing.T,
	sub *header.DummySubscriber,
	bServ blockservice.BlockService,
	startHeight,
	endHeight int,
) {
	sub.Headers = make([]*header.ExtendedHeader, endHeight-startHeight)

	index := 0
	for i := startHeight; i < endHeight; i++ {
		dah := share.RandFillBS(t, 16, bServ)

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

	brokenHeight   int64
	brokenHeightCh chan struct{}

	head    int64
	headers map[int64]*header.ExtendedHeader
}

func (m *mockGetter) generateHeaders(t *testing.T, bServ blockservice.BlockService, startHeight, endHeight int) {
	for i := startHeight; i < endHeight; i++ {
		dah := share.RandFillBS(t, 16, bServ)

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
		switch int64(height) {
		case m.brokenHeight:
			close(m.brokenHeightCh)
		case m.head:
			close(m.doneCh)
		}
	}()

	return m.headers[int64(height)], nil
}

func (m *mockGetter) GetRangeByHeight(ctx context.Context, from, to uint64) ([]*header.ExtendedHeader, error) {
	return nil, nil
}

func (m *mockGetter) Get(context.Context, tmbytes.HexBytes) (*header.ExtendedHeader, error) {
	return nil, nil
}
