package light

import (
	"context"
	_ "embed"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestSharesAvailableCaches(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	getter, eh := GetterWithRandSquare(t, 16)
	dah := eh.DAH
	avail := TestAvailability(getter)

	// cache doesn't have dah yet
	has, err := avail.ds.Has(ctx, rootKey(dah))
	require.NoError(t, err)
	require.False(t, has)

	err = avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)

	// is now stored success result
	result, err := avail.ds.Get(ctx, rootKey(dah))
	require.NoError(t, err)
	failed, err := decodeSamples(result)
	require.NoError(t, err)
	require.Empty(t, failed)
}

func TestSharesAvailableHitsCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	getter, _ := GetterWithRandSquare(t, 16)
	avail := TestAvailability(getter)

	// create new dah, that is not available by getter
	bServ := ipld.NewMemBlockservice()
	dah := availability_test.RandFillBS(t, 16, bServ)
	eh := headertest.RandExtendedHeaderWithRoot(t, dah)

	// blockstore doesn't actually have the dah
	err := avail.SharesAvailable(ctx, eh)
	require.ErrorIs(t, err, share.ErrNotAvailable)

	// put success result in cache
	err = avail.ds.Put(ctx, rootKey(dah), []byte{})
	require.NoError(t, err)

	// should hit cache after putting
	err = avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)
}

func TestSharesAvailableEmptyRoot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	getter, _ := GetterWithRandSquare(t, 16)
	avail := TestAvailability(getter)

	eh := headertest.RandExtendedHeaderWithRoot(t, share.EmptyRoot())
	err := avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)
}

func TestSharesAvailableFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	getter, _ := GetterWithRandSquare(t, 16)
	avail := TestAvailability(getter)

	// create new dah, that is not available by getter
	bServ := ipld.NewMemBlockservice()
	dah := availability_test.RandFillBS(t, 16, bServ)
	eh := headertest.RandExtendedHeaderWithRoot(t, dah)

	// blockstore doesn't actually have the dah, so it should fail
	err := avail.SharesAvailable(ctx, eh)
	require.ErrorIs(t, err, share.ErrNotAvailable)

	// cache should have failed results now
	result, err := avail.ds.Get(ctx, rootKey(dah))
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

func TestShareAvailableOverMocknet_Light(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	net := availability_test.NewTestDAGNet(ctx, t)
	_, root := RandNode(net, 16)
	eh := headertest.RandExtendedHeader(t)
	eh.DAH = root
	nd := Node(net)
	net.ConnectAll()

	err := nd.SharesAvailable(ctx, eh)
	require.NoError(t, err)
}

func TestGetShare(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := 16
	getter, eh := GetterWithRandSquare(t, n)

	for i := range make([]bool, n) {
		for j := range make([]bool, n) {
			sh, err := getter.GetShare(ctx, eh, i, j)
			require.NotNil(t, sh)
			require.NoError(t, err)
		}
	}
}

func TestService_GetSharesByNamespace(t *testing.T) {
	tests := []struct {
		squareSize         int
		expectedShareCount int
	}{
		{squareSize: 4, expectedShareCount: 2},
		{squareSize: 16, expectedShareCount: 2},
		{squareSize: 128, expectedShareCount: 2},
	}

	for _, tt := range tests {
		t.Run("size: "+strconv.Itoa(tt.squareSize), func(t *testing.T) {
			getter, bServ := EmptyGetter()
			totalShares := tt.squareSize * tt.squareSize
			randShares := sharetest.RandShares(t, totalShares)
			idx1 := (totalShares - 1) / 2
			idx2 := totalShares / 2
			if tt.expectedShareCount > 1 {
				// make it so that two rows have the same namespace
				copy(share.GetNamespace(randShares[idx2]), share.GetNamespace(randShares[idx1]))
			}
			root := availability_test.FillBS(t, bServ, randShares)
			eh := headertest.RandExtendedHeader(t)
			eh.DAH = root
			randNamespace := share.GetNamespace(randShares[idx1])

			shares, err := getter.GetSharesByNamespace(context.Background(), eh, randNamespace)
			require.NoError(t, err)
			require.NoError(t, shares.Verify(root, randNamespace))
			flattened := shares.Flatten()
			require.Len(t, flattened, tt.expectedShareCount)
			for _, value := range flattened {
				require.Equal(t, randNamespace, share.GetNamespace(value))
			}
			if tt.expectedShareCount > 1 {
				// idx1 is always smaller than idx2
				require.Equal(t, randShares[idx1], flattened[0])
				require.Equal(t, randShares[idx2], flattened[1])
			}
		})
		t.Run("last two rows of a 4x4 square that have the same namespace have valid NMT proofs", func(t *testing.T) {
			squareSize := 4
			totalShares := squareSize * squareSize
			getter, bServ := EmptyGetter()
			randShares := sharetest.RandShares(t, totalShares)
			lastNID := share.GetNamespace(randShares[totalShares-1])
			for i := totalShares / 2; i < totalShares; i++ {
				copy(share.GetNamespace(randShares[i]), lastNID)
			}
			root := availability_test.FillBS(t, bServ, randShares)
			eh := headertest.RandExtendedHeader(t)
			eh.DAH = root

			shares, err := getter.GetSharesByNamespace(context.Background(), eh, lastNID)
			require.NoError(t, err)
			require.NoError(t, shares.Verify(root, lastNID))
		})
	}
}

func TestGetShares(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := 16
	getter, eh := GetterWithRandSquare(t, n)

	eds, err := getter.GetEDS(ctx, eh)
	require.NoError(t, err)
	gotDAH, err := share.NewRoot(eds)
	require.NoError(t, err)

	require.True(t, eh.DAH.Equals(gotDAH))
}

func TestService_GetSharesByNamespaceNotFound(t *testing.T) {
	getter, eh := GetterWithRandSquare(t, 1)
	eh.DAH.RowRoots = nil

	emptyShares, err := getter.GetSharesByNamespace(context.Background(), eh, sharetest.RandV0Namespace())
	require.NoError(t, err)
	require.Empty(t, emptyShares.Flatten())
}

func BenchmarkService_GetSharesByNamespace(b *testing.B) {
	tests := []struct {
		amountShares int
	}{
		{amountShares: 4},
		{amountShares: 16},
		{amountShares: 128},
	}

	for _, tt := range tests {
		b.Run(strconv.Itoa(tt.amountShares), func(b *testing.B) {
			t := &testing.T{}
			getter, eh := GetterWithRandSquare(t, tt.amountShares)
			root := eh.DAH
			randNamespace := root.RowRoots[(len(root.RowRoots)-1)/2][:share.NamespaceSize]
			root.RowRoots[(len(root.RowRoots) / 2)] = root.RowRoots[(len(root.RowRoots)-1)/2]
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := getter.GetSharesByNamespace(context.Background(), eh, randNamespace)
				require.NoError(t, err)
			}
		})
	}
}
