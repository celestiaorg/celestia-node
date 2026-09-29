package eds

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// TestStreamedResponsesDoNotPopulateCache is the guard behind streaming: the store hands the shrex
// server a cache-backed accessor, and the cache keeps an extended row plus its proofs for every row
// it serves. Without the context opt-out a streamed response retains the whole square anyway, which
// is both far more than the server reserves and outlives the stream, since accessors are pooled.
func TestStreamedResponsesDoNotPopulateCache(t *testing.T) {
	ctx := context.Background()
	const odsSize = 16
	namespace := libshare.RandomNamespace()
	randEDS, _ := edstest.RandEDSWithNamespace(t, namespace, odsSize*odsSize, odsSize)

	cachedRows := func(c *proofsCache) int {
		c.axisCacheLock.RLock()
		defer c.axisCacheLock.RUnlock()
		return len(c.axisCache[0]) + len(c.axisCache[1])
	}

	newCache := func() *proofsCache {
		return WithProofsCache(&Rsmt2D{ExtendedDataSquare: randEDS}).(*proofsCache)
	}

	drain := func(t *testing.T, r io.Reader, err error) {
		t.Helper()
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, r)
		require.NoError(t, err)
		require.NoError(t, r.(io.Closer).Close())
	}

	t.Run("namespace data", func(t *testing.T) {
		c := newCache()
		id, err := shwap.NewNamespaceDataID(1, namespace)
		require.NoError(t, err)
		r, err := id.ResponseReader(ctx, c)
		drain(t, r, err)
		require.Zero(t, cachedRows(c))
	})

	t.Run("range namespace data", func(t *testing.T) {
		c := newCache()
		edsID, err := shwap.NewEdsID(1)
		require.NoError(t, err)
		id, err := shwap.NewRangeNamespaceDataID(edsID, 0, odsSize*odsSize, odsSize)
		require.NoError(t, err)
		r, err := id.ResponseReader(ctx, c)
		drain(t, r, err)
		require.Zero(t, cachedRows(c))
	})

	// the same reads without the opt-out do fill the cache, so the assertions above mean something.
	t.Run("without opt-out", func(t *testing.T) {
		c := newCache()
		_, err := c.RowNamespaceData(ctx, namespace, 0)
		require.NoError(t, err)
		_, err = c.AxisHalf(ctx, 0, 1)
		require.NoError(t, err)
		require.Equal(t, 2, cachedRows(c))
	})
}
