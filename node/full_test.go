package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-core/abci/example/kvstore"
	core_node "github.com/celestiaorg/celestia-core/node"
	rpctest "github.com/celestiaorg/celestia-core/rpc/test"
	"github.com/celestiaorg/celestia-node/node/p2p"
	"github.com/celestiaorg/celestia-node/rpc"
)

func TestNewFull(t *testing.T) {
	coreNode := startCoreNode()
	endpoint := coreNode.Config().RPC.ListenAddress
	protocol, ip := endpoint[:3], endpoint[6:]
	t.Cleanup(func() {
		//nolint:errcheck
		coreNode.Stop()
	})

	nd, err := NewFull(&Config{
		P2P: &p2p.Config{},
		RPC: &rpc.Config{
			Protocol:   protocol,
			RemoteAddr: ip,
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, nd)
	assert.NotNil(t, nd.Config)
	assert.NotZero(t, nd.Type)
}

func TestFullLifecycle(t *testing.T) {
	startCtx, startCtxCancel := context.WithCancel(context.Background())

	coreNode := startCoreNode()
	endpoint := coreNode.Config().RPC.ListenAddress
	protocol, ip := endpoint[:3], endpoint[6:]

	node, err := NewFull(&Config{
		P2P: &p2p.Config{},
		RPC: &rpc.Config{
			Protocol:   protocol,
			RemoteAddr: ip,
		},
	})
	assert.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, node.Config)
	require.NotZero(t, node.Type)
	require.NotNil(t, node.RPCClient)

	err = node.Start(startCtx)
	require.NoError(t, err)

	stopCtx, stopCtxCancel := context.WithCancel(context.Background())
	//nolint:errcheck
	t.Cleanup(func() {
		coreNode.Stop()
		startCtxCancel()
		stopCtxCancel()
	})

	err = node.Stop(stopCtx)
	assert.NoError(t, err)
}

func startCoreNode() *core_node.Node {
	app := kvstore.NewApplication()
	app.RetainBlocks = 10
	return rpctest.StartTendermint(app, rpctest.SuppressStdout)
}
