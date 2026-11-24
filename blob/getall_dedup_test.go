package blob

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// countingGetter wraps a shwap.Getter. It counts GetNamespaceData calls
// and records the highest number of calls that run at the same time.
type countingGetter struct {
	shwap.Getter
	calls    atomic.Int64
	inFlight atomic.Int64
	maxSeen  atomic.Int64
	delay    time.Duration
}

func (g *countingGetter) GetNamespaceData(
	ctx context.Context,
	h *header.ExtendedHeader,
	ns libshare.Namespace,
) (shwap.NamespaceData, error) {
	g.calls.Add(1)
	cur := g.inFlight.Add(1)
	defer g.inFlight.Add(-1)
	for {
		prev := g.maxSeen.Load()
		if cur <= prev || g.maxSeen.CompareAndSwap(prev, cur) {
			break
		}
	}
	if g.delay > 0 {
		time.Sleep(g.delay)
	}
	return g.Getter.GetNamespaceData(ctx, h, ns)
}

func TestService_getAll_DeduplicatesNamespaces(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	libBlobs, err := libshare.GenerateV0Blobs([]int{4, 6}, true)
	require.NoError(t, err)
	blobs, err := convertBlobs(libBlobs...)
	require.NoError(t, err)
	shares, err := BlobsToShares(blobs...)
	require.NoError(t, err)
	shares = append(shares, libshare.TailPaddingShares(16-len(shares))...)

	service := createService(ctx, t, shares)
	getter := &countingGetter{Getter: service.shareGetter}
	service.shareGetter = getter

	ns := blobs[0].Namespace()
	const repeats = 1000
	namespaces := make([]libshare.Namespace, repeats)
	for i := range namespaces {
		namespaces[i] = ns
	}

	hdr, err := service.headerServ.GetByHeight(ctx, 1)
	require.NoError(t, err)
	got, err := service.getAll(ctx, hdr, namespaces)
	require.NoError(t, err)
	// Each blob must be returned once, not once per repeated namespace.
	require.Len(t, got, len(blobs))
	for i := range blobs {
		require.True(t, blobs[i].compareCommitments(got[i].Commitment))
	}
	// The namespace must be fetched once.
	require.EqualValues(t, 1, getter.calls.Load())
}

func TestService_getAll_PreservesNamespaceOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	libBlobs, err := libshare.GenerateV0Blobs([]int{3, 3, 3}, false)
	require.NoError(t, err)
	blobs, err := convertBlobs(libBlobs...)
	require.NoError(t, err)
	shares, err := BlobsToShares(blobs...)
	require.NoError(t, err)
	service := createService(ctx, t, shares)

	ns0, ns1, ns2 := blobs[0].Namespace(), blobs[1].Namespace(), blobs[2].Namespace()
	hdr, err := service.headerServ.GetByHeight(ctx, 1)
	require.NoError(t, err)
	got, err := service.getAll(ctx, hdr, []libshare.Namespace{ns2, ns0, ns2, ns1, ns0})
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.True(t, got[0].Namespace().Equals(ns2))
	require.True(t, got[1].Namespace().Equals(ns0))
	require.True(t, got[2].Namespace().Equals(ns1))
}

func TestService_GetAll_BoundsConcurrency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	libBlobs, err := libshare.GenerateV0Blobs([]int{1}, false)
	require.NoError(t, err)
	blobs, err := convertBlobs(libBlobs...)
	require.NoError(t, err)
	shares, err := BlobsToShares(blobs...)
	require.NoError(t, err)

	service := createService(ctx, t, shares)
	getter := &countingGetter{Getter: service.shareGetter, delay: 5 * time.Millisecond}
	service.shareGetter = getter

	// Distinct namespaces that hold no data.
	const count = 4 * maxConcurrentNamespaceRequests
	namespaces := make([]libshare.Namespace, count)
	for i := range namespaces {
		namespaces[i] = libshare.RandomBlobNamespace()
	}

	_, err = service.GetAll(ctx, 1, namespaces)
	require.NoError(t, err)

	require.EqualValues(t, count, getter.calls.Load())
	require.LessOrEqual(t, getter.maxSeen.Load(), int64(maxConcurrentNamespaceRequests))
}
