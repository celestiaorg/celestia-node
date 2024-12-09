package light

import (
	"context"
	_ "embed"
	"encoding/json"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/ipfs/boxo/bitswap/client"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p/core/host"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/getters/mock"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
)

func TestSharesAvailableSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	square := edstest.RandEDS(t, 16)
	roots, err := share.NewAxisRoots(square)
	require.NoError(t, err)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)

	getter := mock.NewMockGetter(gomock.NewController(t))
	getter.EXPECT().
		GetSamples(gomock.Any(), eh, gomock.Any()).
		DoAndReturn(
			func(_ context.Context, hdr *header.ExtendedHeader, indices []shwap.SampleCoords) ([]shwap.Sample, error) {
				acc := eds.Rsmt2D{ExtendedDataSquare: square}
				smpls := make([]shwap.Sample, len(indices))
				for i, idx := range indices {
					smpl, err := acc.Sample(ctx, idx)
					if err != nil {
						return nil, err
					}

					smpls[i] = smpl
				}
				return smpls, nil
			}).
		AnyTimes()

	ds := datastore.NewMapDatastore()
	avail := NewShareAvailability(getter, ds, nil)

	// Ensure the datastore doesn't have the sampling result yet
	has, err := avail.ds.Has(ctx, datastoreKeyForRoot(roots))
	require.NoError(t, err)
	require.False(t, has)

	err = avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)

	// Verify that the sampling result is stored with all samples marked as available
	data, err := avail.ds.Get(ctx, datastoreKeyForRoot(roots))
	require.NoError(t, err)

	var result SamplingResult
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	require.Empty(t, result.Remaining)
	require.Len(t, result.Available, int(avail.params.SampleAmount))
}

func TestSharesAvailableSkipSampled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a getter that always returns ErrNotFound
	getter := mock.NewMockGetter(gomock.NewController(t))
	getter.EXPECT().
		GetSamples(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, shrex.ErrNotFound).
		AnyTimes()

	ds := datastore.NewMapDatastore()
	avail := NewShareAvailability(getter, ds, nil)

	// generate random header
	roots := edstest.RandomAxisRoots(t, 16)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)

	// store doesn't actually have the eds
	err := avail.SharesAvailable(ctx, eh)
	require.ErrorIs(t, err, share.ErrNotAvailable)

	// Store a successful sampling result in the datastore
	samplingResult := &SamplingResult{
		Available: make([]shwap.SampleCoords, avail.params.SampleAmount),
		Remaining: []shwap.SampleCoords{},
	}
	data, err := json.Marshal(samplingResult)
	require.NoError(t, err)
	err = avail.ds.Put(ctx, datastoreKeyForRoot(roots), data)
	require.NoError(t, err)

	// SharesAvailable should now return nil since the success sampling result is stored
	err = avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)
}

func TestSharesAvailableEmptyEDS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	getter := mock.NewMockGetter(gomock.NewController(t))
	ds := datastore.NewMapDatastore()
	avail := NewShareAvailability(getter, ds, nil)

	// request for empty eds
	eh := headertest.RandExtendedHeaderWithRoot(t, share.EmptyEDSRoots())
	err := avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)
}

func TestSharesAvailableFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type test struct {
		eh    *header.ExtendedHeader
		roots *share.AxisRoots
		eds   *rsmt2d.ExtendedDataSquare
	}

	tests := make([]test, 0)

	for i := 1; i <= 128; i *= 4 {
		// Create new eds, that is not available by getter
		eds := edstest.RandEDS(t, i)
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)

		tests = append(tests, test{eh: eh, roots: roots, eds: eds})
	}

	for _, tt := range tests {
		failGetter := mock.NewMockGetter(gomock.NewController(t))
		ds := datastore.NewMapDatastore()
		avail := NewShareAvailability(failGetter, ds, nil)

		// Getter doesn't have the eds, so it should fail for all samples
		mockSamples := min(int(avail.params.SampleAmount), 2*len(tt.eh.DAH.RowRoots))
		failGetter.EXPECT().
			GetSamples(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(make([]shwap.Sample, mockSamples), shrex.ErrNotFound).
			AnyTimes()

		err := avail.SharesAvailable(ctx, tt.eh)
		require.ErrorIs(t, err, share.ErrNotAvailable)

		// The datastore should now contain the sampling result with all samples in Remaining
		data, err := avail.ds.Get(ctx, datastoreKeyForRoot(tt.roots))
		require.NoError(t, err)

		var failed SamplingResult
		err = json.Unmarshal(data, &failed)
		require.NoError(t, err)

		require.Empty(t, failed.Available)
		if len(tt.roots.RowRoots)*len(tt.roots.RowRoots) < int(avail.params.SampleAmount) {
			require.Len(t, failed.Remaining, len(tt.roots.RowRoots)*len(tt.roots.RowRoots))
		} else {
			require.Len(t, failed.Remaining, int(avail.params.SampleAmount))
		}

		// Simulate a getter that now returns shares successfully
		successfulGetter := newSuccessGetter()
		avail.getter = successfulGetter

		// should be able to retrieve all the failed samples now
		err = avail.SharesAvailable(ctx, tt.eh)
		require.NoError(t, err)

		// The sampling result should now have all samples in Available
		data, err = avail.ds.Get(ctx, datastoreKeyForRoot(tt.roots))
		require.NoError(t, err)

		var result SamplingResult
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		require.Empty(t, result.Remaining)
		if len(tt.roots.RowRoots)*len(tt.roots.RowRoots) < int(avail.params.SampleAmount) {
			require.Len(t, result.Available, len(tt.roots.RowRoots)*len(tt.roots.RowRoots))
		} else {
			require.Len(t, result.Available, int(avail.params.SampleAmount))
		}

		// onceGetter should have no more samples stored after the call
		require.ElementsMatch(t, failed.Remaining, successfulGetter.sampledList())
		successfulGetter.checkOnce(t)
	}
}

func TestParallelAvailability(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ds := datastore.NewMapDatastore()
	// Simulate a getter that returns shares successfully
	successfulGetter := newSuccessGetter()
	avail := NewShareAvailability(successfulGetter, ds, nil)

	// create new eds, that is not available by getter
	eds := edstest.RandEDS(t, 16)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)

	var wg sync.WaitGroup
	const iters = 100
	wg.Add(iters)
	for i := 0; i < iters; i++ {
		go func() {
			defer wg.Done()
			err := avail.SharesAvailable(ctx, eh)
			require.NoError(t, err)
		}()
	}
	wg.Wait()
	require.Len(t, successfulGetter.sampledList(), int(avail.params.SampleAmount))
	successfulGetter.checkOnce(t)

	// Verify that the sampling result is stored with all samples marked as available
	resultData, err := avail.ds.Get(ctx, datastoreKeyForRoot(roots))
	require.NoError(t, err)

	var samplingResult SamplingResult
	err = json.Unmarshal(resultData, &samplingResult)
	require.NoError(t, err)

	require.Empty(t, samplingResult.Remaining)
	require.Len(t, samplingResult.Available, int(avail.params.SampleAmount))
}

type successGetter struct {
	*sync.Mutex
	sampled map[shwap.SampleCoords]int
}

func newSuccessGetter() successGetter {
	return successGetter{
		Mutex:   &sync.Mutex{},
		sampled: make(map[shwap.SampleCoords]int),
	}
}

func (g successGetter) sampledList() []shwap.SampleCoords {
	g.Lock()
	defer g.Unlock()
	return slices.Collect(maps.Keys(g.sampled))
}

func (g successGetter) checkOnce(t *testing.T) {
	g.Lock()
	defer g.Unlock()
	for s, count := range g.sampled {
		if count > 1 {
			t.Errorf("sample %v was called more than once", s)
		}
	}
}

func (g successGetter) GetSamples(_ context.Context, hdr *header.ExtendedHeader,
	indices []shwap.SampleCoords,
) ([]shwap.Sample, error) {
	g.Lock()
	defer g.Unlock()

	smpls := make([]shwap.Sample, 0, len(indices))
	for _, idx := range indices {
		s := shwap.SampleCoords{Row: idx.Row, Col: idx.Col}
		g.sampled[s]++
		smpls = append(smpls, shwap.Sample{Proof: &nmt.Proof{}})
	}
	return smpls, nil
}

func (g successGetter) GetEDS(_ context.Context, _ *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error) {
	panic("not implemented")
}

func (g successGetter) GetNamespaceData(
	_ context.Context,
	_ *header.ExtendedHeader,
	_ libshare.Namespace,
) (shwap.NamespaceData, error) {
	panic("not implemented")
}

func TestPruneAll(t *testing.T) {
	const size = 8
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	t.Cleanup(cancel)

	eds, h := randEdsAndHeader(t, size)
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	clientBs := blockstore.NewBlockstore(ds)

	ex := newExchangeOverEDS(ctx, t, eds)
	getter := bitswap.NewGetter(ex, clientBs, 0)
	getter.Start()
	defer getter.Stop()

	// Create a new ShareAvailability instance and sample the shares
	sampleAmount := uint(20)
	avail := NewShareAvailability(getter, ds, clientBs, WithSampleAmount(sampleAmount))
	err := avail.SharesAvailable(ctx, h)
	require.NoError(t, err)
	// close ShareAvailability to force flush of batched writes
	avail.Close(ctx)

	preDeleteCount := countKeys(ctx, t, clientBs)
	require.EqualValues(t, sampleAmount, preDeleteCount)

	// prune the samples
	err = avail.Prune(ctx, h)
	require.NoError(t, err)

	// Check if samples are deleted
	postDeleteCount := countKeys(ctx, t, clientBs)
	require.Zero(t, postDeleteCount)

	// Check if sampling result is deleted
	exist, err := avail.ds.Has(ctx, datastoreKeyForRoot(h.DAH))
	require.NoError(t, err)
	require.False(t, exist)
}

func TestPrunePartialFailed(t *testing.T) {
	const size = 8
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	t.Cleanup(cancel)

	eds, h := randEdsAndHeader(t, size)
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	clientBs := blockstore.NewBlockstore(ds)

	ex := newHalfSessionExchange(newExchangeOverEDS(ctx, t, eds))
	getter := bitswap.NewGetter(ex, clientBs, 0)
	getter.Start()
	defer getter.Stop()

	// Create a new ShareAvailability instance and sample the shares
	sampleAmount := uint(20)
	avail := NewShareAvailability(getter, ds, clientBs, WithSampleAmount(sampleAmount))
	err := avail.SharesAvailable(ctx, h)
	require.Error(t, err)
	// close ShareAvailability to force flush of batched writes
	avail.Close(ctx)

	preDeleteCount := countKeys(ctx, t, clientBs)
	// Only half of the samples should be stored
	require.EqualValues(t, sampleAmount/2, preDeleteCount)

	// prune the samples
	err = avail.Prune(ctx, h)
	require.NoError(t, err)

	// Check if samples are deleted
	postDeleteCount := countKeys(ctx, t, clientBs)
	require.Zero(t, postDeleteCount)

	// Check if sampling result is deleted
	exist, err := avail.ds.Has(ctx, datastoreKeyForRoot(h.DAH))
	require.NoError(t, err)
	require.False(t, exist)
}

func TestPruneWithCancelledContext(t *testing.T) {
	const size = 8
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	t.Cleanup(cancel)

	eds, h := randEdsAndHeader(t, size)
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	clientBs := blockstore.NewBlockstore(ds)

	ex := newTimeoutExchange(newExchangeOverEDS(ctx, t, eds))
	getter := bitswap.NewGetter(ex, clientBs, 0)
	getter.Start()
	defer getter.Stop()

	// Create a new ShareAvailability instance and sample the shares
	sampleAmount := uint(20)
	avail := NewShareAvailability(getter, ds, clientBs, WithSampleAmount(sampleAmount))

	ctx2, cancel2 := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel2()
	go func() {
		// cancel context a bit later.
		time.Sleep(100 * time.Millisecond)
		cancel2()
	}()

	err := avail.SharesAvailable(ctx2, h)
	require.Error(t, err, context.Canceled)
	// close ShareAvailability to force flush of batched writes
	avail.Close(ctx)

	preDeleteCount := countKeys(ctx, t, clientBs)
	require.EqualValues(t, sampleAmount, preDeleteCount)

	// prune the samples
	err = avail.Prune(ctx, h)
	require.NoError(t, err)

	// Check if samples are deleted
	postDeleteCount := countKeys(ctx, t, clientBs)
	require.Zero(t, postDeleteCount)

	// Check if sampling result is deleted
	exist, err := avail.ds.Has(ctx, datastoreKeyForRoot(h.DAH))
	require.NoError(t, err)
	require.False(t, exist)
}

type halfSessionExchange struct {
	exchange.SessionExchange
	attempt atomic.Int32
}

func newHalfSessionExchange(ex exchange.SessionExchange) *halfSessionExchange {
	return &halfSessionExchange{SessionExchange: ex}
}

func (hse *halfSessionExchange) NewSession(context.Context) exchange.Fetcher {
	return hse
}

func (hse *halfSessionExchange) GetBlocks(ctx context.Context, cids []cid.Cid) (<-chan blocks.Block, error) {
	out := make(chan blocks.Block, len(cids))
	defer close(out)

	for _, cid := range cids {
		if hse.attempt.Add(1)%2 == 0 {
			continue
		}

		blk, err := hse.SessionExchange.GetBlock(ctx, cid)
		if err != nil {
			return nil, err
		}

		out <- blk
	}

	return out, nil
}

type timeoutExchange struct {
	exchange.SessionExchange
}

func newTimeoutExchange(ex exchange.SessionExchange) *timeoutExchange {
	return &timeoutExchange{SessionExchange: ex}
}

func (hse *timeoutExchange) NewSession(context.Context) exchange.Fetcher {
	return hse
}

func (hse *timeoutExchange) GetBlocks(ctx context.Context, cids []cid.Cid) (<-chan blocks.Block, error) {
	out := make(chan blocks.Block, len(cids))
	defer close(out)

	for _, cid := range cids {
		blk, err := hse.SessionExchange.GetBlock(ctx, cid)
		if err != nil {
			return nil, err
		}

		out <- blk
	}

	// sleep guarantees that we context will be canceled in a test.
	time.Sleep(200 * time.Millisecond)

	return out, nil
}

func randEdsAndHeader(t *testing.T, size int) (*rsmt2d.ExtendedDataSquare, *header.ExtendedHeader) {
	height := uint64(42)
	eds := edstest.RandEDS(t, size)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)

	h := &header.ExtendedHeader{
		RawHeader: header.RawHeader{
			Height: int64(height),
			Time:   time.Now(),
		},
		DAH: roots,
	}
	return eds, h
}

func countKeys(ctx context.Context, t *testing.T, bs blockstore.Blockstore) int {
	keys, err := bs.AllKeysChan(ctx)
	require.NoError(t, err)
	var count int
	for range keys {
		count++
	}
	return count
}

func newExchangeOverEDS(ctx context.Context, t *testing.T, rsmt2d *rsmt2d.ExtendedDataSquare) exchange.SessionExchange {
	bstore := &bitswap.Blockstore{
		Getter: testAccessorGetter{
			AccessorStreamer: &eds.Rsmt2D{ExtendedDataSquare: rsmt2d},
		},
	}
	return newExchange(ctx, t, bstore)
}

func newExchange(ctx context.Context, t *testing.T, bstore blockstore.Blockstore) exchange.SessionExchange {
	net, err := mocknet.FullMeshLinked(3)
	require.NoError(t, err)

	newServer(ctx, net.Hosts()[0], bstore)
	newServer(ctx, net.Hosts()[1], bstore)

	client := newClient(ctx, net.Hosts()[2], bstore)

	err = net.ConnectAllButSelf()
	require.NoError(t, err)
	return client
}

func newServer(ctx context.Context, host host.Host, store blockstore.Blockstore) {
	net := bitswap.NewNetwork(host, "test")
	server := bitswap.NewServer(
		ctx,
		net,
		store,
	)
	net.Start(server)
}

func newClient(ctx context.Context, host host.Host, store blockstore.Blockstore) *client.Client {
	net := bitswap.NewNetwork(host, "test")
	client := bitswap.NewClient(ctx, net, store)
	net.Start(client)
	return client
}

type testAccessorGetter struct {
	eds.AccessorStreamer
}

func (t testAccessorGetter) GetByHeight(context.Context, uint64) (eds.AccessorStreamer, error) {
	return t.AccessorStreamer, nil
}

func (t testAccessorGetter) HasByHeight(context.Context, uint64) (bool, error) {
	return true, nil
}
