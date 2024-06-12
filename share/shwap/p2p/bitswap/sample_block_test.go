package bitswap

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestSampleRoundtrip_GetContainers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	eds := edstest.RandEDS(t, 8)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)
	client := fetcher(ctx, t, eds)

	width := int(eds.Width())
	blks := make([]Block, 0, width*width)
	for x := 0; x < width; x++ {
		for y := 0; y < width; y++ {
			blk, err := NewEmptySampleBlock(1, x, y, root)
			require.NoError(t, err)
			blks = append(blks, blk)
		}
	}

	err = Fetch(ctx, client, root, blks...)
	require.NoError(t, err)

	for _, sample := range blks {
		blk := sample.(*SampleBlock)
		err = blk.Container().Validate(root, blk.ID.RowIndex, blk.ID.ShareIndex)
		require.NoError(t, err)
	}
}
