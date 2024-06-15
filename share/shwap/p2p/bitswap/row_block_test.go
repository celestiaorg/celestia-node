package bitswap

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestRowRoundtrip_GetContainers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	eds := edstest.RandEDS(t, 4)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)
	client := fetcher(ctx, t, eds)

	blks := make([]Block, eds.Width())
	for i := range blks {
		blk, err := NewEmptyRowBlock(1, i, root)
		require.NoError(t, err)
		blks[i] = blk
	}

	err = Fetch(ctx, client, root, blks...)
	require.NoError(t, err)

	for _, blk := range blks {
		row := blk.(*RowBlock)
		err = row.Container.Validate(root, row.ID.RowIndex)
		require.NoError(t, err)
	}
}
