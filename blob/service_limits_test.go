package blob

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/blob/mocks"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	gettermocks "github.com/celestiaorg/celestia-node/share/shwap/getters/mock"
)

func TestServiceGetAll_RequestLimit(t *testing.T) {
	for _, count := range []int{0, 1, 64, 65} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			hs := mocks.NewMockHeaderService(ctrl)
			service := NewService(nil, nil, hs, nil)
			namespaces := make([]libshare.Namespace, count)
			for i := range namespaces {
				namespaces[i] = libshare.RandomNamespace()
			}
			headerErr := errors.New("header unavailable")
			if count <= 64 {
				hs.EXPECT().GetByHeight(gomock.Any(), uint64(1)).Return(nil, headerErr)
			}
			got, err := service.GetAll(context.Background(), 1, namespaces)
			require.Nil(t, got)
			if count > 64 {
				require.ErrorContains(t, err, "maximum 64")
			} else {
				require.ErrorIs(t, err, headerErr)
			}
		})
	}
}

func TestServiceGetAll_ConcurrencyAndCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctrl := gomock.NewController(t)
		getter := gettermocks.NewMockGetter(ctrl)
		service := NewService(nil, getter, nil, nil)
		hdr := headertest.RandExtendedHeader(t)
		namespaces := make([]libshare.Namespace, 64)
		for i := range namespaces {
			namespaces[i] = libshare.RandomNamespace()
		}
		var calls atomic.Int32
		getter.EXPECT().GetNamespaceData(gomock.Any(), hdr, gomock.Any()).AnyTimes().
			DoAndReturn(func(ctx context.Context, _ *header.ExtendedHeader, _ libshare.Namespace) (shwap.NamespaceData, error) {
				calls.Add(1)
				<-ctx.Done()
				return nil, ctx.Err()
			})
		done := make(chan error, 1)
		go func() {
			_, err := service.getAll(ctx, hdr, namespaces)
			done <- err
		}()
		synctest.Wait()
		require.EqualValues(t, 16, calls.Load())
		cancel()
		synctest.Wait()
		require.ErrorIs(t, <-done, context.Canceled)
		require.EqualValues(t, 16, calls.Load(), "queued fetches must not start after cancellation")
	})
}

func TestServiceGetAll_RejectDuplicateNamespaces(t *testing.T) {
	ns := libshare.RandomNamespace()
	duplicate, err := libshare.NewNamespaceFromBytes(append([]byte(nil), ns.Bytes()...))
	require.NoError(t, err)
	for _, namespaces := range [][]libshare.Namespace{
		{ns, duplicate},
		{ns, libshare.RandomNamespace(), duplicate},
	} {
		ctrl := gomock.NewController(t)
		hs := mocks.NewMockHeaderService(ctrl)
		service := NewService(nil, nil, hs, nil)
		blobs, err := service.GetAll(context.Background(), 1, namespaces)
		require.ErrorContains(t, err, "duplicate namespace")
		require.Nil(t, blobs)
	}
}

func TestServiceGetAll_PartialResultsAcrossBatches(t *testing.T) {
	sizes := make([]int, 64)
	for i := range sizes {
		sizes[i] = 1
	}
	libBlobs, err := libshare.GenerateV0Blobs(sizes, false)
	require.NoError(t, err)
	blobs, err := convertBlobs(libBlobs...)
	require.NoError(t, err)
	shares, err := BlobsToShares(blobs...)
	require.NoError(t, err)
	ctx := context.Background()
	service := createService(ctx, t, shares)
	namespaces := make([]libshare.Namespace, len(blobs))
	for i := range namespaces {
		namespaces[i] = blobs[len(blobs)-1-i].Namespace()
	}
	fetchErr := errors.New("namespace fetch failed")
	inner := service.shareGetter
	getter := gettermocks.NewMockGetter(gomock.NewController(t))
	getter.EXPECT().GetNamespaceData(gomock.Any(), gomock.Any(), gomock.Any()).Times(len(namespaces)).
		DoAndReturn(func(ctx context.Context, hdr *header.ExtendedHeader,
			ns libshare.Namespace,
		) (shwap.NamespaceData, error) {
			if ns.Equals(namespaces[0]) {
				return nil, fetchErr
			}
			return inner.GetNamespaceData(ctx, hdr, ns)
		})
	service.shareGetter = getter
	got, err := service.GetAll(ctx, 1, namespaces)
	require.ErrorIs(t, err, fetchErr)
	require.Len(t, got, len(namespaces)-1)
	for i, b := range got {
		require.True(t, b.Namespace().Equals(namespaces[i+1]))
		require.Equal(t, blobs[len(blobs)-2-i].Data(), b.Data())
	}
}
