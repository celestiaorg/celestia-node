package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/node/p2p"
	"github.com/celestiaorg/celestia-node/node/rpc"
	"github.com/celestiaorg/celestia-node/testutils"
)

func TestNewFull(t *testing.T) {
	coreNode, protocol, ip := testutils.StartMockCoreNode()
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

	coreNode, protocol, ip := testutils.StartMockCoreNode()

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
