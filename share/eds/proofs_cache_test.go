package eds

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestCache(t *testing.T) {
	ODSSize := 16
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	newAccessor := func(tb testing.TB, inner *rsmt2d.ExtendedDataSquare) Accessor {
		accessor := &Rsmt2D{ExtendedDataSquare: inner}
		return WithProofsCache(accessor)
	}
	TestSuiteAccessor(ctx, t, newAccessor, ODSSize)

	newAccessorStreamer := func(tb testing.TB, inner *rsmt2d.ExtendedDataSquare) AccessorStreamer {
		accessor := &Rsmt2D{ExtendedDataSquare: inner}
		return WithProofsCache(accessor)
	}
	TestStreamer(ctx, t, newAccessorStreamer, ODSSize)
}

func TestProofsCacheIsBounded(t *testing.T) {
	ctx := context.Background()
	square := edstest.RandEDS(t, maxCachedAxes)
	acc := WithProofsCache(&Rsmt2D{ExtendedDataSquare: square}).(*proofsCache)

	width := int(square.Width())
	for i := range width {
		_, err := acc.Sample(ctx, shwap.SampleCoords{Row: i, Col: 0})
		require.NoError(t, err)
		_, err = acc.AxisHalf(ctx, rsmt2d.Col, i)
		require.NoError(t, err)
	}
	require.Equal(t, maxCachedAxes, acc.axisCache[rsmt2d.Row].Len())
	require.Equal(t, maxCachedAxes, acc.axisCache[rsmt2d.Col].Len())

	// evicted axes are rebuilt on demand
	_, err := acc.Sample(ctx, shwap.SampleCoords{Row: 0, Col: 0})
	require.NoError(t, err)
}
