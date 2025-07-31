package bitswap

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestSample_FetchRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	eds := edstest.RandEDS(t, 32)
	root, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	exchange := newExchangeOverEDS(ctx, t, eds)

	width := int(eds.Width())
	blks := make([]Block, 0, width*width)
	for x := range width {
		for y := range width {
			idx := shwap.SampleCoords{Row: x, Col: y}

			blk, err := NewEmptySampleBlock(1, idx, len(root.RowRoots))
			require.NoError(t, err)
			blks = append(blks, blk)
		}
	}

	err = Fetch(ctx, exchange, root, blks)
	require.NoError(t, err)

	for _, sample := range blks {
		blk := sample.(*SampleBlock)
		err = blk.Container.Verify(root, blk.ID.RowIndex, blk.ID.ShareIndex)
		require.NoError(t, err)
	}
}
