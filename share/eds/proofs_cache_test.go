package eds

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
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
	square := edstest.RandEDS(t, 128)
	acc := WithProofsCache(&Rsmt2D{ExtendedDataSquare: square}).(*proofsCache)

	width := int(square.Width())
	limit := axisCacheSize(width)
	require.Less(t, limit, width)
	for i := range width {
		_, err := acc.axisWithProofs(ctx, rsmt2d.Row, i)
		require.NoError(t, err)
		_, err = acc.AxisHalf(ctx, rsmt2d.Col, i)
		require.NoError(t, err)
	}
	require.Equal(t, limit, cachedAxes(acc, rsmt2d.Row))
	require.Equal(t, limit, cachedAxes(acc, rsmt2d.Col))

	// evicted axes are rebuilt on demand
	_, err := acc.axisWithProofs(ctx, rsmt2d.Row, 0)
	require.NoError(t, err)
}

func TestProofsCacheSampleMissDoesNotCache(t *testing.T) {
	ctx := context.Background()
	square := edstest.RandEDS(t, 8)
	acc := WithProofsCache(&Rsmt2D{ExtendedDataSquare: square}).(*proofsCache)

	roots, err := share.NewAxisRoots(square)
	require.NoError(t, err)
	for i := range int(square.Width()) {
		idx := shwap.SampleCoords{Row: i, Col: i}
		smpl, err := acc.Sample(ctx, idx)
		require.NoError(t, err)
		require.NoError(t, smpl.Verify(roots, idx.Row, idx.Col))
	}
	require.Zero(t, cachedAxes(acc, rsmt2d.Row))

	// a row cached with proofs serves samples from the cache
	_, err = acc.axisWithProofs(ctx, rsmt2d.Row, 0)
	require.NoError(t, err)
	require.Equal(t, 1, cachedAxes(acc, rsmt2d.Row))
	idx := shwap.SampleCoords{Row: 0, Col: 1}
	smpl, err := acc.Sample(ctx, idx)
	require.NoError(t, err)
	require.NoError(t, smpl.Verify(roots, idx.Row, idx.Col))
}

func cachedAxes(c *proofsCache, axisType rsmt2d.Axis) int {
	if c.axisCache == nil {
		return 0
	}
	return c.axisCache[axisType].Len()
}
