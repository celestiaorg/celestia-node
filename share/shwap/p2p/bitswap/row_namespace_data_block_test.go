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

func TestRowNamespaceDataRoundtrip_GetContainers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	namespace := sharetest.RandV0Namespace()
	eds, root := edstest.RandEDSWithNamespace(t, namespace, 64, 16)
	client := fetcher(ctx, t, newTestBlockstore(eds))

	rowIdxs := share.RowsWithNamespace(root, namespace)
	blks := make([]Block, len(rowIdxs))
	for i, rowIdx := range rowIdxs {
		blk, err := NewEmptyRowNamespaceDataBlock(1, rowIdx, namespace, root)
		require.NoError(t, err)
		blks[i] = blk
	}

	err := Fetch(ctx, client, root, blks...)
	require.NoError(t, err)

	for _, blk := range blks {
		rnd := blk.(*RowNamespaceDataBlock)
		err = rnd.Container.Validate(root, rnd.ID.DataNamespace, rnd.ID.RowIndex)
		require.NoError(t, err)
	}
}
