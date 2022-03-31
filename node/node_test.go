package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"
)

func Test_StateServiceConstruction(t *testing.T) {
	for _, tp := range []Type{Bridge, Full, Light} {
		nd, _, err := core.StartRemoteClient()
		require.NoError(t, err)
		node := TestNode(t, tp, WithRemoteCore(core.GetRemoteEndpoint(nd)))
		// check to ensure node's state service is not nil
		require.NotNil(t, node.StateServ)
		// stop the node
		require.NoError(t, node.Stop(context.Background()))
		require.NoError(t, nd.Stop())
	}
}
