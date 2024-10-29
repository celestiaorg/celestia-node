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
	result, err := avail.ds.Get(ctx, datastoreKeyForRoot(roots))
	require.NoError(t, err)

	var failed []Sample
	err = json.Unmarshal(result, &failed)
	require.NoError(t, err)
	require.Empty(t, failed)
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
	failed := []Sample{}
	data, err := json.Marshal(failed)
	require.NoError(t, err)
	err = avail.ds.Put(ctx, datastoreKeyForRoot(roots), data)
	require.NoError(t, err)

	// SharesAvailable should now return no error since the success sampling result is stored
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
	result, err := avail.ds.Get(ctx, datastoreKeyForRoot(roots))
	require.NoError(t, err)

	var failed []Sample
	err = json.Unmarshal(result, &failed)
	require.NoError(t, err)
	require.Len(t, failed, int(avail.params.SampleAmount))

	// Simulate a getter that now returns shares successfully
	successfulGetter := newOnceGetter()
	successfulGetter.AddSamples(failed)
	avail.getter = successfulGetter

	// should be able to retrieve all the failed samples now
	err = avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)

	// onceGetter should have no more samples stored after the call
	require.Empty(t, successfulGetter.available)
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

func (m onceGetter) GetShare(_ context.Context, _ *header.ExtendedHeader, row, col int) (libshare.Share, error) {
	m.Lock()
	defer m.Unlock()
	s := Sample{Row: row, Col: col}
	if _, ok := m.available[s]; ok {
		delete(m.available, s)
		return libshare.Share{}, nil
	}
	return libshare.Share{}, share.ErrNotAvailable
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
