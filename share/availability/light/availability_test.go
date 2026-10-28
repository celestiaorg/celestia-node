package light

import (
	"context"
	_ "embed"
	"encoding/json"
	"maps"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/getters/mock"
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

// TestSharesAvailablePartialResponse verifies that when a getter returns a
// length-preserving slice with one not-retrieved (empty) sample, that sample's
// coordinate is recorded in Remaining and the rest in Available — i.e. results
// are matched to coords by position. A getter that compacted out empty samples
// (shorter slice) would misattribute coordinates and silently drop the failed
// one from Remaining.
func TestSharesAvailablePartialResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	square := edstest.RandEDS(t, 16)
	roots, err := share.NewAxisRoots(square)
	require.NoError(t, err)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)

	var missing shwap.SampleCoords
	getter := mock.NewMockGetter(gomock.NewController(t))
	getter.EXPECT().
		GetSamples(gomock.Any(), eh, gomock.Any()).
		DoAndReturn(
			func(_ context.Context, _ *header.ExtendedHeader, indices []shwap.SampleCoords) ([]shwap.Sample, error) {
				acc := eds.Rsmt2D{ExtendedDataSquare: square}
				smpls := make([]shwap.Sample, len(indices))
				for i, idx := range indices {
					// leave the first requested coord empty to simulate a
					// not-retrieved sample, keeping the slice length intact.
					if i == 0 {
						missing = idx
						continue
					}
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
	avail := NewShareAvailability(getter, ds)

	// the missing sample makes availability incomplete.
	err = avail.SharesAvailable(ctx, eh)
	require.Error(t, err)

	data, err := avail.ds.Get(ctx, datastoreKeyForRoot(roots))
	require.NoError(t, err)
	var result SamplingResult
	require.NoError(t, json.Unmarshal(data, &result))

	// the empty sample's coordinate must be the one recorded as remaining.
	require.Contains(t, result.Remaining, missing)
	require.NotContains(t, result.Available, missing)
	require.Len(t, result.Remaining, 1)
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
	avail := NewShareAvailability(getter, ds)

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
	avail := NewShareAvailability(getter, ds)

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
		avail := NewShareAvailability(failGetter, ds)

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
	avail := NewShareAvailability(successfulGetter, ds)

	// create new eds, that is not available by getter
	eds := edstest.RandEDS(t, 16)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)

	var wg sync.WaitGroup
	const iters = 100
	wg.Add(iters)
	for range iters {
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

func (g successGetter) GetSamples(_ context.Context, _ *header.ExtendedHeader,
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

func (g successGetter) GetRow(_ context.Context, _ *header.ExtendedHeader, _ int) (shwap.Row, error) {
	panic("not implemented")
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

func (g successGetter) GetRangeNamespaceData(
	_ context.Context,
	_ *header.ExtendedHeader,
	_, _ int,
) (shwap.RangeNamespaceData, error) {
	panic("not implemented")
}

func TestPrune(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	t.Cleanup(cancel)

	roots, err := share.NewAxisRoots(edstest.RandEDS(t, 8))
	require.NoError(t, err)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)

	avail := NewShareAvailability(newSuccessGetter(), datastore.NewMapDatastore())
	require.NoError(t, avail.SharesAvailable(ctx, eh))
	require.NoError(t, avail.Close(ctx))

	exist, err := avail.ds.Has(ctx, datastoreKeyForRoot(roots))
	require.NoError(t, err)
	require.True(t, exist)

	require.NoError(t, avail.Prune(ctx, eh))

	exist, err = avail.ds.Has(ctx, datastoreKeyForRoot(roots))
	require.NoError(t, err)
	require.False(t, exist)
}

func TestRemoveLegacySamples(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	t.Cleanup(cancel)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	legacy := 3*writeBatchSize + 1
	for i := range legacy {
		key := legacySamplesPrefix.ChildString(strconv.Itoa(i))
		require.NoError(t, ds.Put(ctx, key, []byte("sample")))
	}
	unrelated := datastore.NewKey("blocksync/head")
	require.NoError(t, ds.Put(ctx, unrelated, []byte("keep")))

	require.NoError(t, RemoveLegacySamples(ctx, ds))

	res, err := ds.Query(ctx, query.Query{Prefix: legacySamplesPrefix.String(), KeysOnly: true})
	require.NoError(t, err)
	left, err := res.Rest()
	require.NoError(t, err)
	require.Empty(t, left)

	has, err := ds.Has(ctx, unrelated)
	require.NoError(t, err)
	require.True(t, has)

	require.NoError(t, RemoveLegacySamples(ctx, ds))
}
