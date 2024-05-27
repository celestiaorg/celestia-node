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

func TestRowRoundtrip_GetContainers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	eds := edstest.RandEDS(t, 4)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)
	client := remoteClient(ctx, t, newTestBlockstore(eds))

	ids := make([]ID, eds.Width())
	data := make([]*RowBlock, eds.Width())
	for i := range ids {
		rid, err := shwap.NewRowID(1, i, root)
		require.NoError(t, err)
		log.Debugf("%X", RowID(rid).CID())
		data[i] = &RowBlock{RowID: RowID(rid)}
		ids[i] = data[i]
	}

	err = GetContainers(ctx, client, root, ids...)
	require.NoError(t, err)

	for _, row := range data {
		err = row.Row.Validate(root, row.RowIndex)
		require.NoError(t, err)
	}

	var entries int
	globalVerifiers.Range(func(any, any) bool {
		entries++
		return true
	})
	require.Zero(t, entries)
}
