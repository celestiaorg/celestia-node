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
	require.Zero(t, len(acc.axisCache[0]))
	eid, err := shwap.NewEdsID(1)
	require.NoError(t, err)
	rid, err := shwap.NewRangeNamespaceDataID(eid, 1, 16*16-1, 16)
	require.NoError(t, err)
	r, err = rid.ResponseReader(ctx, acc)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, r)
	require.NoError(t, err)
	require.Zero(t, len(acc.axisCache[0]))
	// Ordinary reads must still use the shared cache.
	_, err = acc.RowNamespaceData(ctx, ns, 0)
	require.NoError(t, err)
	require.Equal(t, 1, len(acc.axisCache[0]))
}
