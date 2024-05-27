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
	nds := make([]*RowNamespaceDataBlock, len(rowIdxs))
	ids := make([]ID, len(rowIdxs))
	for i, rowIdx := range rowIdxs {
		rid, err := shwap.NewRowNamespaceDataID(1, rowIdx, namespace, root)
		require.NoError(t, err)
		//TODO(@walldiss): not sure if copy of RowNamespaceDataID type is needed in bitswap
		nds[i] = &RowNamespaceDataBlock{RowNamespaceDataID: RowNamespaceDataID(rid)}
		ids[i] = nds[i]
	}

	err := GetContainers(ctx, client, root, ids...)
	require.NoError(t, err)

	for _, nd := range nds {
		err = nd.Data.Validate(root, nd.RowNamespaceDataID.DataNamespace, nd.RowNamespaceDataID.RowIndex)
		require.NoError(t, err)
	}
}
