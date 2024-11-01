package light

import (
	"context"
	_ "embed"
	"encoding/json"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/ipfs/go-datastore"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/getters/mock"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
)

func TestSharesAvailableSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eds := edstest.RandEDS(t, 16)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)

	getter := mock.NewMockGetter(gomock.NewController(t))
	getter.EXPECT().
		GetShare(gomock.Any(), eh, gomock.Any(), gomock.Any()).
		DoAndReturn(
			func(_ context.Context, _ *header.ExtendedHeader, row, col int) (libshare.Share, error) {
				rawSh := eds.GetCell(uint(row), uint(col))
				sh, err := libshare.NewShare(rawSh)
				if err != nil {
					return libshare.Share{}, err
				}
				return *sh, nil
			}).
		AnyTimes()

	ds := datastore.NewMapDatastore()
	avail := NewShareAvailability(getter, ds)

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
		GetShare(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(libshare.Share{}, shrex.ErrNotFound).
		AnyTimes()

	ds := datastore.NewMapDatastore()
	avail := NewShareAvailability(getter, ds)

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
	avail := NewShareAvailability(getter, ds)

	// request for empty eds
	eh := headertest.RandExtendedHeaderWithRoot(t, share.EmptyEDSRoots())
	err := avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)
}

func TestSharesAvailableFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	getter := mock.NewMockGetter(gomock.NewController(t))
	ds := datastore.NewMapDatastore()
	avail := NewShareAvailability(getter, ds)

	// Create new eds, that is not available by getter
	eds := edstest.RandEDS(t, 16)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)

	// Getter doesn't have the eds, so it should fail for all samples
	getter.EXPECT().
		GetShare(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(libshare.Share{}, shrex.ErrNotFound).
		AnyTimes()
	err = avail.SharesAvailable(ctx, eh)
	require.ErrorIs(t, err, share.ErrNotAvailable)

	// The datastore should now contain the sampling result with all samples in Remaining
	data, err := avail.ds.Get(ctx, datastoreKeyForRoot(roots))
	require.NoError(t, err)

	var failed SamplingResult
	err = json.Unmarshal(data, &failed)
	require.NoError(t, err)

	require.Empty(t, failed.Available)
	require.Len(t, failed.Remaining, int(avail.params.SampleAmount))

	// Simulate a getter that now returns shares successfully
	successfulGetter := newOnceGetter()
	avail.getter = successfulGetter

	// should be able to retrieve all the failed samples now
	err = avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)

	// The sampling result should now have all samples in Available
	data, err = avail.ds.Get(ctx, datastoreKeyForRoot(roots))
	require.NoError(t, err)

	var result SamplingResult
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	require.Empty(t, result.Remaining)
	require.Len(t, result.Available, int(avail.params.SampleAmount))

	// onceGetter should have no more samples stored after the call
	successfulGetter.checkOnce(t)
	require.ElementsMatch(t, failed.Remaining, successfulGetter.sampledList())
}

func TestParallelAvailability(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ds := datastore.NewMapDatastore()
	// Simulate a getter that returns shares successfully
	successfulGetter := newOnceGetter()
	avail := NewShareAvailability(successfulGetter, ds)

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
	require.Len(t, successfulGetter.sampledList(), int(avail.params.SampleAmount))

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

func (g onceGetter) sampledList() []Sample {
	g.Lock()
	defer g.Unlock()
	samples := make([]Sample, 0, len(g.sampled))
	for s := range g.sampled {
		samples = append(samples, s)
	}
	return samples
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

func (g onceGetter) GetSharesByNamespace(
	_ context.Context,
	_ *header.ExtendedHeader,
	_ libshare.Namespace,
) (shwap.NamespaceData, error) {
	panic("not implemented")
}
