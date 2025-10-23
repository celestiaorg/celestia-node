package bitswap

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v3/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestRowNamespaceData_FetchRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	namespace := libshare.RandomNamespace()
	eds, root := edstest.RandEDSWithNamespace(t, namespace, 64, 16)
	exchange := newExchangeOverEDS(ctx, t, eds)

	rowIdxs, err := share.RowsWithNamespace(root, namespace)
	require.NoError(t, err)
	blks := make([]Block, len(rowIdxs))
	for i, rowIdx := range rowIdxs {
		blk, err := NewEmptyRowNamespaceDataBlock(1, rowIdx, namespace, len(root.RowRoots))
		require.NoError(t, err)
		blks[i] = blk
	}

	err = Fetch(ctx, exchange, root, blks)
	require.NoError(t, err)

	for _, blk := range blks {
		rnd := blk.(*RowNamespaceDataBlock)
		err = rnd.Container.Verify(root, rnd.ID.DataNamespace, rnd.ID.RowIndex)
		require.NoError(t, err)
	}
}
