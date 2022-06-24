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

	// 15 headers from the past and 15 future headers
	mockGet, shareServ, sub, mockService := createDASerSubcomponents(t, bServ, 15, 15, share.NewLightAvailability)

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

	// give catch-up routine a second to finish up sampling last header
	// TODO @renaynay: this sleep is a known flakey solution to the issue that
	//  we do not have DASState implemented yet. Once DASState is implemented, rely
	//  on that instead of sleeping.
	time.Sleep(time.Second * 1)

	err = daser.Stop(ctx)
	require.NoError(t, err)

	// load checkpoint and ensure it's at network head
	checkpoint, err := loadCheckpoint(ctx, daser.cstore)
	require.NoError(t, err)
	// ensure checkpoint is stored at 15
	assert.Equal(t, int64(15), checkpoint)
}

func TestDASer_Restart(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	bServ := mdutils.Bserv()

	// 15 headers from the past and 15 future headers
	mockGet, shareServ, sub, mockService := createDASerSubcomponents(t, bServ, 15, 15, share.NewLightAvailability)

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
		t.Fatal(ctx.Err())
	case <-mockGet.doneCh:
	}

	// give catch-up routine a second to finish up sampling last header
	// TODO @renaynay: this sleep is a known flakey solution to the issue that
	//  we do not have DASState implemented yet. Once DASState is implemented, rely
	//  on that instead of sleeping.
	time.Sleep(time.Second * 1)

	err = daser.Stop(restartCtx)
	require.NoError(t, err)

	// load checkpoint and ensure it's at network head
	checkpoint, err := loadCheckpoint(ctx, daser.cstore)
	require.NoError(t, err)
	// ensure checkpoint is stored at 45
	assert.Equal(t, int64(45), checkpoint)
}

func TestDASer_catchUp(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	bServ := mdutils.Bserv()

	mockGet, shareServ, _, mockService := createDASerSubcomponents(t, bServ, 5, 0, share.NewLightAvailability)

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

	mockGet, shareServ, _, mockService := createDASerSubcomponents(t, bServ, 6, 0, share.NewLightAvailability)
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

	mockGet, _, _, mockService := createDASerSubcomponents(t, bServ, 6, 0, share.NewLightAvailability)
	daser := NewDASer(share.NewBrokenAvailability(), nil, mockGet, ds, mockService)

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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
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
	// 15 headers from the past and 15 future headers
	mockGet, shareServ, sub, _ := createDASerSubcomponents(t, bServ, 15, 15, share.NewFullAvailability)

	// create fraud service and break one header
	f := fraud.NewService(ps, mockGet.GetByHeight)
	require.NoError(t, f.RegisterUnmarshaler(fraud.BadEncoding, fraud.UnmarshalBEFP))
	mockGet.headers[1] = header.CreateFraudExtHeader(t, mockGet.headers[1], bServ)

	// create and start DASer
	daser := NewDASer(shareServ, sub, mockGet, ds, f)
	go fraud.OnBEFP(ctx, f, daser.Stop)
	require.NoError(t, daser.Start(ctx))

	// wait for dasing catch-up routine fails
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-daser.ctx.Done():
		break
	}

	// give catch-up routine a second to finish up sampling last header
	// TODO @renaynay: this sleep is a known flakey solution to the issue that
	//  we do not have DASState implemented yet. Once DASState is implemented, rely
	//  on that instead of sleeping.
	time.Sleep(time.Second * 1)
	require.True(t, daser.ctx.Err() == context.Canceled)
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
	availabilityFn func(blockservice.BlockService) share.Availability,
) (*mockGetter, *share.Service, *header.DummySubscriber, *fraud.DummyService) {
	shareServ := share.NewService(bServ, availabilityFn(bServ))

	mockGet := &mockGetter{
		headers: make(map[int64]*header.ExtendedHeader),
		doneCh:  make(chan struct{}),
	}

	mockGet.generateHeaders(t, bServ, 0, numGetter)

	sub := new(header.DummySubscriber)
	fraud := new(fraud.DummyService)
	mockGet.fillSubWithHeaders(t, sub, bServ, numGetter, numGetter+numSub)

	return mockGet, shareServ, sub, fraud
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
		if int64(height) == m.head {
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
