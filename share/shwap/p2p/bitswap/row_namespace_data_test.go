package bitswap

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestRowNamespaceDataRoundtrip_GetContainers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	namespace := sharetest.RandV0Namespace()
	eds, root := edstest.RandEDSWithNamespace(t, namespace, 64, 16)
	client := remoteClient(ctx, t, newTestBlockstore(eds))

	rowIdxs := share.RowsWithNamespace(root, namespace)
	ids := make([]ID[shwap.RowNamespaceData], len(rowIdxs))
	for i, rowIdx := range rowIdxs {
		rid, err := shwap.NewRowNamespaceDataID(1, rowIdx, namespace, root)
		require.NoError(t, err)
		ids[i] = RowNamespaceDataID(rid)
	}

	cntrs, err := GetContainers(ctx, client, root, ids...)
	require.NoError(t, err)
	require.Len(t, cntrs, len(ids))

	for i, cntr := range cntrs {
		sid := ids[i].(RowNamespaceDataID)
		err = cntr.Validate(root, sid.DataNamespace, sid.RowIndex)
		require.NoError(t, err)
	}
}
