package bitswap

import (
	"context"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSampleRoundtrip_GetContainers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	eds := edstest.RandEDS(t, 8)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)
	client := remoteClient(ctx, t, newTestBlockstore(eds))

	width := int(eds.Width())
	ids := make([]ID, 0, width*width)
	samples := make([]*SampleBlock, 0, width*width)
	for x := 0; x < width; x++ {
		for y := 0; y < width; y++ {
			sid, err := shwap.NewSampleID(1, x, y, root)
			require.NoError(t, err)
			sampleBlock := &SampleBlock{SampleID: SampleID(sid)}
			ids = append(ids, sampleBlock)
			samples = append(samples, sampleBlock)
		}
	}

	err = GetContainers(ctx, client, root, ids...)
	require.NoError(t, err)

	for _, sample := range samples {
		err = sample.Sample.Validate(root, sample.RowIndex, sample.ShareIndex)
		require.NoError(t, err)
	}
}
