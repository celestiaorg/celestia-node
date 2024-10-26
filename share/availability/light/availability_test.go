package light

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/boxo/exchange/offline"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
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
	"github.com/celestiaorg/celestia-node/store"
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
			func(_ context.Context, hdr *header.ExtendedHeader, indices []shwap.SampleIndex) ([]shwap.Sample, error) {
				acc := eds.Rsmt2D{ExtendedDataSquare: square}
				smpls := make([]shwap.Sample, len(indices))
				for i, idx := range indices {
					rowIdx, colIdx, err := idx.Coordinates(len(hdr.DAH.RowRoots))
					if err != nil {
						return nil, err
					}

					smpl, err := acc.Sample(ctx, rowIdx, colIdx)
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
		Available: make([]Sample, avail.params.SampleAmount),
		Remaining: []Sample{},
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

	failGetter := mock.NewMockGetter(gomock.NewController(t))
	// Getter doesn't have the eds, so it should fail for all samples
	// failGetter.EXPECT().
	// 	GetShare(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
	// 	Return(libshare.Share{}, shrex.ErrNotFound).
	failGetter.EXPECT().
		GetSamples(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(make([]shwap.Sample, DefaultSampleAmount), shrex.ErrNotFound).
		AnyTimes()

	ds := datastore.NewMapDatastore()
	avail := NewShareAvailability(failGetter, ds, nil)

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
		avail.getter = failGetter

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
		successfulGetter := newOnceGetter()
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
		successfulGetter.checkOnce(t)
		// require.ElementsMatch(t, failed.Remaining, successfulGetter.sampledList())
	}
	// // onceGetter should have no more samples stored after the call
	// successfulGetter.checkOnce(t)
	// require.ElementsMatch(t, failed.Remaining, len(successfulGetter.sampled))
}

func TestParallelAvailability(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ds := datastore.NewMapDatastore()
	// Simulate a getter that returns shares successfully
	successfulGetter := newOnceGetter()
	avail := NewShareAvailability(successfulGetter, ds, nil)

	// create new eds, that is not available by getter
	eds := edstest.RandEDS(t, 16)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := avail.SharesAvailable(ctx, eh)
			require.NoError(t, err)
		}()
	}
	wg.Wait()
	require.Len(t, len(successfulGetter.sampled), int(avail.params.SampleAmount))

	// Verify that the sampling result is stored with all samples marked as available
	resultData, err := avail.ds.Get(ctx, datastoreKeyForRoot(roots))
	require.NoError(t, err)

	var samplingResult SamplingResult
	err = json.Unmarshal(resultData, &samplingResult)
	require.NoError(t, err)

	require.Empty(t, samplingResult.Remaining)
	require.Len(t, samplingResult.Available, int(avail.params.SampleAmount))
}

type onceGetter struct {
	*sync.Mutex
	sampled map[Sample]int
}

func newOnceGetter() onceGetter {
	return onceGetter{
		Mutex:   &sync.Mutex{},
		sampled: make(map[Sample]int),
	}
}

func (g onceGetter) checkOnce(t *testing.T) {
	g.Lock()
	defer g.Unlock()
	for s, count := range g.sampled {
		if count > 1 {
			t.Errorf("sample %v was called more than once", s)
		}
	}
}

func (m onceGetter) GetSamples(_ context.Context, hdr *header.ExtendedHeader, indices []shwap.SampleIndex) ([]shwap.Sample, error) {
	m.Lock()
	defer m.Unlock()

	smpls := make([]shwap.Sample, 0, len(indices))
	for _, idx := range indices {
		rowIdx, colIdx, err := idx.Coordinates(len(hdr.DAH.RowRoots))
		if err != nil {
			return nil, err
		}

		s := Sample{Row: rowIdx, Col: colIdx}
		if _, ok := m.sampled[s]; ok {
			delete(m.sampled, s)
			smpls = append(smpls, shwap.Sample{Proof: &nmt.Proof{}})
		}
	}
	return smpls, nil
}

func (g onceGetter) GetShare(_ context.Context, _ *header.ExtendedHeader, row, col int) (libshare.Share, error) {
	g.Lock()
	defer g.Unlock()
	s := Sample{Row: row, Col: col}
	g.sampled[s]++
	return libshare.Share{}, nil
}

func (g onceGetter) GetEDS(_ context.Context, _ *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error) {
	panic("not implemented")
}

func (g onceGetter) GetNamespaceData(
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

	dir := t.TempDir()
	store, err := store.NewStore(store.DefaultParameters(), dir)
	require.NoError(t, err)
	defer require.NoError(t, store.Stop(ctx))
	eds, h := randEdsAndHeader(t, size)
	err = store.PutODSQ4(ctx, h.DAH, h.Height(), eds)
	require.NoError(t, err)

	// Create a new bitswap getter
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	clientBs := blockstore.NewBlockstore(ds)
	serverBS := &bitswap.Blockstore{Getter: store}
	ex := newFakeExchange(serverBS)
	getter := bitswap.NewGetter(ex, clientBs, 0)
	getter.Start()
	defer getter.Stop()

	// Create a new ShareAvailability instance and sample the shares
	sampleAmount := uint(20)
	avail := NewShareAvailability(getter, ds, clientBs, WithSampleAmount(sampleAmount))
	err = avail.SharesAvailable(ctx, h)
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

	dir := t.TempDir()
	store, err := store.NewStore(store.DefaultParameters(), dir)
	require.NoError(t, err)
	defer require.NoError(t, store.Stop(ctx))
	eds, h := randEdsAndHeader(t, size)
	err = store.PutODSQ4(ctx, h.DAH, h.Height(), eds)
	require.NoError(t, err)

	// Create a new bitswap getter
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	clientBs := blockstore.NewBlockstore(ds)
	serverBS := newHalfFailBlockstore(&bitswap.Blockstore{Getter: store})
	ex := newFakeExchange(serverBS)
	getter := bitswap.NewGetter(ex, clientBs, 0)
	getter.Start()
	defer getter.Stop()

	// Create a new ShareAvailability instance and sample the shares
	sampleAmount := uint(20)
	avail := NewShareAvailability(getter, ds, clientBs, WithSampleAmount(sampleAmount))
	err = avail.SharesAvailable(ctx, h)
	require.NoError(t, err)
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

var _ exchange.SessionExchange = (*fakeSessionExchange)(nil)

func newFakeExchange(bs blockstore.Blockstore) *fakeSessionExchange {
	return &fakeSessionExchange{
		Interface: offline.Exchange(bs),
		session:   offline.Exchange(bs),
	}
}

type fakeSessionExchange struct {
	exchange.Interface
	session exchange.Fetcher
}

func (fe *fakeSessionExchange) NewSession(context.Context) exchange.Fetcher {
	return fe.session
}

type halfFailBlockstore struct {
	blockstore.Blockstore
	attempt atomic.Int32
}

func newHalfFailBlockstore(bs blockstore.Blockstore) *halfFailBlockstore {
	return &halfFailBlockstore{Blockstore: bs}
}

func (hfb *halfFailBlockstore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	if hfb.attempt.Add(1)%2 == 0 {
		return nil, errors.New("fail")
	}
	return hfb.Blockstore.Get(ctx, c)
}

func randEdsAndHeader(t *testing.T, size int) (*rsmt2d.ExtendedDataSquare, *header.ExtendedHeader) {
	height := uint64(42)
	eds := edstest.RandEDS(t, size)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)

	h := &header.ExtendedHeader{
		RawHeader: header.RawHeader{
			Height: int64(height),
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
