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

func TestSampleRoundtrip_GetContainers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	eds := edstest.RandEDS(t, 8)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)
	client := remoteClient(ctx, t, newTestBlockstore(eds))

	var ids []ID[shwap.Sample]
	width := int(eds.Width())
	for x := 0; x < width; x++ {
		for y := 0; y < width; y++ {
			sid, err := shwap.NewSampleID(1, x, y, root)
			require.NoError(t, err)
			ids = append(ids, SampleID(sid))
		}
	}

	cntrs, err := GetContainers(ctx, client, root, ids...)
	require.NoError(t, err)
	require.Len(t, cntrs, len(ids))

	for i, cntr := range cntrs {
		sid := ids[i].(SampleID)
		err = cntr.Validate(root, sid.RowIndex, sid.ShareIndex)
		require.NoError(t, err)
	}
}
