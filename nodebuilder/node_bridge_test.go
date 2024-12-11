package nodebuilder

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-node/core"
	coremodule "github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

func TestBridge_WithMockedCoreClient(t *testing.T) {
	t.Skip("skipping") // consult https://github.com/celestiaorg/celestia-core/issues/667 for reasoning
	repo := MockStore(t, DefaultConfig(node.Bridge))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	_, _, err := net.SplitHostPort(core.StartTestNode(t).GRPCClient.Target())
	require.NoError(t, err)
	con, err := coremodule.NewGRPCClient(
		core.StartTestNode(t).GRPCClient.Target(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	con.Connect()
	require.NoError(t, err)
	node, err := New(node.Bridge, p2p.Private, repo, coremodule.WithConnection(con))
	require.NoError(t, err)
	require.NotNil(t, node)
	err = node.Start(ctx)
	require.NoError(t, err)

	err = node.Stop(ctx)
	require.NoError(t, err)
}
