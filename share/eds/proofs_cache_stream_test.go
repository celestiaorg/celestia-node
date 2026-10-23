package eds

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/rsmt2d"
	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestNamespaceResponseReusesCachedProofs(t *testing.T) {
	ctx := context.Background()
	ns := libshare.RandomNamespace()
	square, _ := edstest.RandEDSWithNamespace(t, ns, 4*4, 4)
	inner := WithClosedOnce(&Rsmt2D{ExtendedDataSquare: square})
	acc := WithProofsCache(inner).(*proofsCache)
	_, err := acc.Sample(ctx, shwap.SampleCoords{Row: 0, Col: 0})
	require.NoError(t, err)
	want, err := acc.RowNamespaceData(ctx, ns, 0)
	require.NoError(t, err)
	_, err = acc.AxisHalf(ctx, rsmt2d.Row, 1)
	require.NoError(t, err)

	// Cache hits must work without reading the underlying accessor.
	require.NoError(t, inner.Close())
	ctx = shwap.WithCacheDisabled(ctx)
	got, err := acc.RowNamespaceData(ctx, ns, 0)
	require.NoError(t, err)
	require.Equal(t, want, got)
	// Rows without proofs must use the underlying accessor without filling the cache.
	for _, row := range []int{1, 2} {
		_, err = acc.RowNamespaceData(ctx, ns, row)
		require.ErrorIs(t, err, errAccessorClosed)
	}
	require.Equal(t, 2, cachedAxes(acc, rsmt2d.Row))
	ax, ok := acc.axisCache[rsmt2d.Row].Peek(1)
	require.True(t, ok)
	require.Nil(t, ax.proofs)
}

func TestNamespaceResponseDoesNotCacheRows(t *testing.T) {
	ctx := context.Background()
	ns := libshare.RandomNamespace()
	square, _ := edstest.RandEDSWithNamespace(t, ns, 16*16, 16)
	acc := WithProofsCache(&Rsmt2D{ExtendedDataSquare: square}).(*proofsCache)
	id, err := shwap.NewNamespaceDataID(1, ns)
	require.NoError(t, err)
	r, err := id.ResponseReader(ctx, acc)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, r)
	require.NoError(t, err)
	require.Zero(t, cachedAxes(acc, rsmt2d.Row))
	eid, err := shwap.NewEdsID(1)
	require.NoError(t, err)
	rid, err := shwap.NewRangeNamespaceDataID(eid, 1, 16*16-1, 16)
	require.NoError(t, err)
	r, err = rid.ResponseReader(ctx, acc)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, r)
	require.NoError(t, err)
	require.Zero(t, cachedAxes(acc, rsmt2d.Row))
	// Ordinary reads must still use the shared cache.
	_, err = acc.RowNamespaceData(ctx, ns, 0)
	require.NoError(t, err)
	require.Equal(t, 1, cachedAxes(acc, rsmt2d.Row))
}
