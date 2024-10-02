package bitswap

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestRowNamespaceData_FetchRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	namespace := sharetest.RandV0Namespace()
	eds, root := edstest.RandEDSWithNamespace(t, namespace, 64, 16)
	exchange := newExchangeOverEDS(ctx, t, eds)

	rowIdxs := share.RowsWithNamespace(root, namespace)
	blks := make([]Block, len(rowIdxs))
	for i, rowIdx := range rowIdxs {
		blk, err := NewEmptyRowNamespaceDataBlock(1, rowIdx, namespace, len(root.RowRoots))
		require.NoError(t, err)
		blks[i] = blk
	}

	err := Fetch(ctx, exchange, root, blks)
	require.NoError(t, err)

	for _, blk := range blks {
		rnd := blk.(*RowNamespaceDataBlock)
		err = rnd.Container.Verify(root, rnd.ID.DataNamespace, rnd.ID.RowIndex)
		require.NoError(t, err)
	}
}
