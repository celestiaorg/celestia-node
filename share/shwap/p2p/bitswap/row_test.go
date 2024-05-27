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

	eds := edstest.RandEDS(t, 2)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)
	client := remoteClient(ctx, t, newTestBlockstore(eds))

	ids := make([]ID[shwap.Row], eds.Width())
	for i := range ids {
		rid, err := shwap.NewRowID(1, i, root)
		require.NoError(t, err)
		log.Debugf("%X", RowID(rid).CID())
		ids[i] = RowID(rid)
	}

	cntrs, err := GetContainers(ctx, client, root, ids...)
	require.NoError(t, err)
	require.Len(t, cntrs, len(ids))

	for i, cntr := range cntrs {
		rid := ids[i].(RowID)
		err = cntr.Validate(root, rid.RowIndex)
		require.NoError(t, err)
	}

	var entries int
	globalVerifiers.Range(func(any, any) bool {
		entries++
		return true
	})
	require.Zero(t, entries)
}
