package light

import (
	"context"
	_ "embed"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/ipfs/go-datastore"
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
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
)

func TestSharesAvailableCaches(t *testing.T) {
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
	avail := NewShareAvailability(getter, ds)

	// cache doesn't have eds yet
	has, err := avail.ds.Has(ctx, rootKey(roots))
	require.NoError(t, err)
	require.False(t, has)

	err = avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)

	// is now stored success result
	result, err := avail.ds.Get(ctx, rootKey(roots))
	require.NoError(t, err)
	failed, err := decodeSamples(result)
	require.NoError(t, err)
	require.Empty(t, failed)
}

func TestSharesAvailableHitsCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create getter that always return ErrNotFound
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

	// put success result in cache
	err = avail.ds.Put(ctx, rootKey(roots), []byte{})
	require.NoError(t, err)

	// should hit cache after putting
	err = avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)
}

func TestSharesAvailableEmptyRoot(t *testing.T) {
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

	// create new eds, that is not available by getter
	eds := edstest.RandEDS(t, 16)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)

	// getter doesn't have the eds, so it should fail
	getter.EXPECT().
		GetSamples(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(make([]shwap.Sample, avail.params.SampleAmount), shrex.ErrNotFound).
		AnyTimes()
	err = avail.SharesAvailable(ctx, eh)
	require.ErrorIs(t, err, share.ErrNotAvailable)

	// cache should have failed results now
	result, err := avail.ds.Get(ctx, rootKey(roots))
	require.NoError(t, err)

	failed, err := decodeSamples(result)
	require.NoError(t, err)
	require.Len(t, failed, int(avail.params.SampleAmount))

	// ensure that retry persists the failed samples selection
	// create new getter with only the failed samples available, and add them to the onceGetter
	onceGetter := newOnceGetter()
	onceGetter.AddSamples(failed)

	// replace getter with the new one
	avail.getter = onceGetter

	// should be able to retrieve all the failed samples now
	err = avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)

	// onceGetter should have no more samples stored after the call
	require.Empty(t, onceGetter.available)
}

type onceGetter struct {
	*sync.Mutex
	available map[Sample]struct{}
}

func newOnceGetter() onceGetter {
	return onceGetter{
		Mutex:     &sync.Mutex{},
		available: make(map[Sample]struct{}),
	}
}

func (m onceGetter) AddSamples(samples []Sample) {
	m.Lock()
	defer m.Unlock()
	for _, s := range samples {
		m.available[s] = struct{}{}
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

		s := Sample{Row: uint16(rowIdx), Col: uint16(colIdx)}
		if _, ok := m.available[s]; ok {
			delete(m.available, s)
			smpls = append(smpls, shwap.Sample{Proof: &nmt.Proof{}})
		}
	}
	return smpls, nil
}

func (m onceGetter) GetEDS(_ context.Context, _ *header.ExtendedHeader) (*rsmt2d.ExtendedDataSquare, error) {
	panic("not implemented")
}

func (m onceGetter) GetSharesByNamespace(
	_ context.Context,
	_ *header.ExtendedHeader,
	_ libshare.Namespace,
) (shwap.NamespaceData, error) {
	panic("not implemented")
}
