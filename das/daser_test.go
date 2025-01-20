package das

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/go-fraud/fraudtest"
	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/mocks"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

var timeout = time.Second * 15

// TestDASerLifecycle tests to ensure every mock block is DASed and
// the DASer checkpoint is updated to network head.
func TestDASerLifecycle(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	ctrl := gomock.NewController(t)
	avail := mocks.NewMockAvailability(ctrl)
	avail.EXPECT().SharesAvailable(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	// 15 headers from the past and 15 future headers
	mockGet, sub, mockService := createDASerSubcomponents(t, 15, 15)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	daser, err := NewDASer(avail, sub, mockGet, ds, mockService, newBroadcastMock(1))
	require.NoError(t, err)

	err = daser.Start(ctx)
	require.NoError(t, err)
	defer func() {
		err = daser.Stop(ctx)
		require.NoError(t, err)

		// load checkpoint and ensure it's at network head
		checkpoint, err := daser.store.load(ctx)
		require.NoError(t, err)
		// ensure checkpoint is stored at 30
		require.EqualValues(t, 30, checkpoint.SampleFrom-1)
	}()

	// wait for mock to indicate that catchup is done
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-mockGet.doneCh:
	}

	// wait for DASer to indicate done
	require.NoError(t, waitHeight(ctx, daser, 30))
}

func TestDASer_Restart(t *testing.T) {
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	ctrl := gomock.NewController(t)
	avail := mocks.NewMockAvailability(ctrl)
	avail.EXPECT().SharesAvailable(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	// 15 headers from the past and 15 future headers
	mockGet, sub, mockService := createDASerSubcomponents(t, 15, 15)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	daser, err := NewDASer(avail, sub, mockGet, ds, mockService, newBroadcastMock(1))
	require.NoError(t, err)

	err = daser.Start(ctx)
	require.NoError(t, err)

	// wait for mock to indicate that catchup is done
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-mockGet.doneCh:
	}

	// wait for DASer to indicate done
	require.NoError(t, waitHeight(ctx, daser, 30))

	err = daser.Stop(ctx)
	require.NoError(t, err)

	// reset mockGet, generate 15 "past" headers, building off chain head which is 30
	mockGet.generateHeaders(t, 30, 45)
	mockGet.doneCh = make(chan struct{})
	// reset dummy subscriber
	mockGet.fillSubWithHeaders(t, sub, 45, 60)
	// manually set mockGet head to trigger finished at 45
	mockGet.head = int64(45)

	// restart DASer with new context
	restartCtx, restartCancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(restartCancel)

	daser, err = NewDASer(avail, sub, mockGet, ds, mockService, newBroadcastMock(1))
	require.NoError(t, err)

	err = daser.Start(restartCtx)
	require.NoError(t, err)

	// wait for dasing catch-up routine to indicateDone
	select {
	case <-restartCtx.Done():
		t.Fatal(restartCtx.Err())
	case <-mockGet.doneCh:
	}

	require.NoError(t, waitHeight(ctx, daser, 60))
	err = daser.Stop(restartCtx)
	require.NoError(t, err)

	// load checkpoint and ensure it's at network head
	checkpoint, err := daser.store.load(ctx)
	require.NoError(t, err)
	// ensure checkpoint is stored at 45
	assert.EqualValues(t, 60, checkpoint.SampleFrom-1)
}

// TODO(@walldiss): BEFP test will not work until BEFP-shwap integration
// func TestDASer_stopsAfter_BEFP(t *testing.T) {
//	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
//	t.Cleanup(cancel)
//
//	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
//	// create mock network
//	net, err := mocknet.FullMeshLinked(1)
//	require.NoError(t, err)
//	// create pubsub for host
//	ps, err := pubsub.NewGossipSub(ctx, net.Hosts()[0],
//		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
//	require.NoError(t, err)
//
//	ctrl := gomock.NewController(t)
//	avail := mocks.NewMockAvailability(ctrl)
//	avail.EXPECT().SharesAvailable(gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
//	// 15 headers from the past and 15 future headers
//	mockGet, sub, _ := createDASerSubcomponents(t, 15, 15)
//
//	// create fraud service and break one header
//	getter := func(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
//		return mockGet.GetByHeight(ctx, height)
//	}
//	headGetter := func(ctx context.Context) (*header.ExtendedHeader, error) {
//		return mockGet.Head(ctx)
//	}
//	unmarshaler := fraud.MultiUnmarshaler[*header.ExtendedHeader]{
//		Unmarshalers: map[fraud.ProofType]func([]byte) (fraud.Proof[*header.ExtendedHeader], error){
//			byzantine.BadEncoding: func(data []byte) (fraud.Proof[*header.ExtendedHeader], error) {
//				befp := &byzantine.BadEncodingProof{}
//				return befp, befp.UnmarshalBinary(data)
//			},
//		},
//	}
//
//	fserv := fraudserv.NewProofService[*header.ExtendedHeader](ps,
//		net.Hosts()[0],
//		getter,
//		headGetter,
//		unmarshaler,
//		ds,
//		false,
//		"private",
//	)
//	require.NoError(t, fserv.Start(ctx))
//	mockGet.headers[1] = headerfraud.CreateFraudExtHeader(t, mockGet.headers[1])
//	newCtx := context.Background()
//
//	// create and start DASer
//	daser, err := NewDASer(avail, sub, mockGet, ds, fserv, newBroadcastMock(1))
//	require.NoError(t, err)
//
//	resultCh := make(chan error)
//	go fraud.OnProof[*header.ExtendedHeader](newCtx, fserv, byzantine.BadEncoding,
//		func(fraud.Proof[*header.ExtendedHeader]) {
//			resultCh <- daser.Stop(newCtx)
//		})
//
//	require.NoError(t, daser.Start(newCtx))
//	// wait for fraud proof will be handled
//	select {
//	case <-ctx.Done():
//		t.Fatal(ctx.Err())
//	case res := <-resultCh:
//		require.NoError(t, res)
//	}
//	// wait for manager to finish catchup
//	require.False(t, daser.running.Load())
//}

func TestDASerSampleTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	getter := getterStub{}
	avail := mocks.NewMockAvailability(gomock.NewController(t))
	doneCh := make(chan struct{})
	avail.EXPECT().SharesAvailable(gomock.Any(), gomock.Any()).AnyTimes().
		DoAndReturn(
			func(sampleCtx context.Context, h *header.ExtendedHeader) error {
				select {
				case <-sampleCtx.Done():
					close(doneCh)
					return nil
				case <-ctx.Done():
					t.Fatal("call context didn't timeout in time")
					return ctx.Err()
				}
			})

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	sub := new(headertest.Subscriber)
	fserv := &fraudtest.DummyService[*header.ExtendedHeader]{}

	// create and start DASer
	daser, err := NewDASer(avail, sub, getter, ds, fserv, newBroadcastMock(1),
		WithSampleTimeout(1))
	require.NoError(t, err)

	require.NoError(t, daser.Start(ctx))
	require.NoError(t, daser.sampler.state.waitCatchUp(ctx))

	select {
	case <-doneCh:
	case <-ctx.Done():
		t.Fatal("call context didn't timeout in time")
	}
}

// createDASerSubcomponents takes numGetter (number of headers
// to store in mockGetter) and numSub (number of headers to store
// in the mock header.Subscriber), returning a newly instantiated
// mockGetter, share.Availability, and mock header.Subscriber.
func createDASerSubcomponents(
	t *testing.T,
	numGetter,
	numSub int,
) (*mockGetter, *headertest.Subscriber, *fraudtest.DummyService[*header.ExtendedHeader]) {
	mockGet, sub := createMockGetterAndSub(t, numGetter, numSub)
	fraud := &fraudtest.DummyService[*header.ExtendedHeader]{}
	return mockGet, sub, fraud
}

func createMockGetterAndSub(
	t *testing.T,
	numGetter,
	numSub int,
) (*mockGetter, *headertest.Subscriber) {
	mockGet := &mockGetter{
		headers:        make(map[int64]*header.ExtendedHeader),
		doneCh:         make(chan struct{}),
		brokenHeightCh: make(chan struct{}),
	}

	mockGet.generateHeaders(t, 0, numGetter)

	sub := new(headertest.Subscriber)
	mockGet.fillSubWithHeaders(t, sub, numGetter, numGetter+numSub)
	return mockGet, sub
}

// fillSubWithHeaders generates `num` headers from the future for p2pSub to pipe through to DASer.
func (m *mockGetter) fillSubWithHeaders(
	t *testing.T,
	sub *headertest.Subscriber,
	startHeight,
	endHeight int,
) {
	sub.Headers = make([]*header.ExtendedHeader, endHeight-startHeight)

	index := 0
	for i := startHeight; i < endHeight; i++ {
		roots := edstest.RandomAxisRoots(t, 16)
		randHeader := headertest.RandExtendedHeaderWithRoot(t, roots)
		randHeader.RawHeader.Height = int64(i + 1)

		sub.Headers[index] = randHeader
		// also checkpointStore to mock getter for duplicate sampling
		m.headers[int64(i+1)] = randHeader

		index++
	}
}

type mockGetter struct {
	getterStub
	doneCh chan struct{} // signals all stored headers have been retrieved

	brokenHeight   int64
	brokenHeightCh chan struct{}

	head    int64
	headers map[int64]*header.ExtendedHeader
}

func (m *mockGetter) generateHeaders(t *testing.T, startHeight, endHeight int) {
	for i := startHeight; i < endHeight; i++ {
		roots := edstest.RandomAxisRoots(t, 16)

		randHeader := headertest.RandExtendedHeaderWithRoot(t, roots)
		randHeader.RawHeader.Height = int64(i + 1)

		m.headers[int64(i+1)] = randHeader
	}
	// set network head
	m.head = int64(startHeight + endHeight)
}

func (m *mockGetter) Head(
	context.Context,
	...libhead.HeadOption[*header.ExtendedHeader],
) (*header.ExtendedHeader, error) {
	return m.headers[m.head], nil
}

func (m *mockGetter) GetByHeight(_ context.Context, height uint64) (*header.ExtendedHeader, error) {
	defer func() {
		switch int64(height) {
		case m.brokenHeight:
			select {
			case <-m.brokenHeightCh:
			default:
				close(m.brokenHeightCh)
			}
		case m.head:
			select {
			case <-m.doneCh:
			default:
				close(m.doneCh)
			}
		}
	}()

	if h, ok := m.headers[int64(height)]; ok {
		return h, nil
	}
	return nil, fmt.Errorf("header not found")
}

type benchGetterStub struct {
	getterStub
	header *header.ExtendedHeader
}

func newBenchGetter() benchGetterStub {
	return benchGetterStub{header: &header.ExtendedHeader{
		DAH: &share.AxisRoots{RowRoots: make([][]byte, 0)},
	}}
}

func (m benchGetterStub) GetByHeight(context.Context, uint64) (*header.ExtendedHeader, error) {
	return m.header, nil
}

type getterStub struct{}

func (m getterStub) Head(
	context.Context,
	...libhead.HeadOption[*header.ExtendedHeader],
) (*header.ExtendedHeader, error) {
	return &header.ExtendedHeader{RawHeader: header.RawHeader{Height: 1}}, nil
}

func (m getterStub) GetByHeight(_ context.Context, height uint64) (*header.ExtendedHeader, error) {
	return &header.ExtendedHeader{
		Commit:    &types.Commit{},
		RawHeader: header.RawHeader{Height: int64(height)},
		DAH:       &share.AxisRoots{RowRoots: make([][]byte, 0)},
	}, nil
}

func (m getterStub) GetRangeByHeight(
	context.Context,
	*header.ExtendedHeader,
	uint64,
) ([]*header.ExtendedHeader, error) {
	panic("implement me")
}

func (m getterStub) Get(context.Context, libhead.Hash) (*header.ExtendedHeader, error) {
	panic("implement me")
}

// waitHeight waits for the DASer to catch up to the given height. It will return an error if the
// DASer fails to catch up to the given height within the timeout.
func waitHeight(ctx context.Context, daser *DASer, height uint64) error {
	for {
		err := daser.WaitCatchUp(ctx)
		if err != nil {
			return err
		}
		stats, err := daser.SamplingStats(ctx)
		if err != nil {
			return err
		}
		if stats.SampledChainHead == height {
			return nil
		}
		time.Sleep(time.Millisecond * 100)
	}
}
