//go:build !race

package nodebuilder

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

func TestStorageOnlyMode_P2PDisabled(t *testing.T) {
	consNode := core.StartTestNode(t)
	host, port, err := net.SplitHostPort(consNode.GRPCClient.Target())
	require.NoError(t, err)

	cfg := DefaultConfig(node.Bridge)
	cfg.Core.IP = host
	cfg.Core.Port = port
	cfg.P2P.Disabled = true
	cfg.Share.StoreODSOnly = true

	nd := TestNodeWithConfig(t, node.Bridge, cfg)
	require.NotNil(t, nd)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = nd.Start(ctx)
	require.NoError(t, err)

	require.Nil(t, nd.Host, "P2P Host should be nil when P2P is disabled")
	require.Nil(t, nd.ConnGater, "ConnGater should be nil when P2P is disabled")
	require.Nil(t, nd.Routing, "Routing should be nil when P2P is disabled")
	require.Nil(t, nd.DataExchange, "DataExchange should be nil when P2P is disabled")
	require.Nil(t, nd.BlockService, "BlockService should be nil when P2P is disabled")
	require.Nil(t, nd.PubSub, "PubSub should be nil when P2P is disabled")

	require.NotNil(t, nd.RPCServer, "RPC Server should be available")
	require.NotNil(t, nd.ShareServ, "Share service should be available")
	require.NotNil(t, nd.HeaderServ, "Header service should be available")
	require.NotNil(t, nd.StateServ, "State service should be available")

	err = nd.Stop(ctx)
	require.NoError(t, err)
}

func TestStorageOnlyMode_ValidationFailure(t *testing.T) {
	consNode := core.StartTestNode(t)
	host, port, err := net.SplitHostPort(consNode.GRPCClient.Target())
	require.NoError(t, err)

	cfg := DefaultConfig(node.Bridge)
	cfg.Core.IP = host
	cfg.Core.Port = port
	cfg.P2P.Disabled = true
	cfg.Share.StoreODSOnly = false

	store := MockStore(t, cfg)
	_, err = New(node.Bridge, modp2p.Private, store)
	require.Error(t, err)
	require.Contains(t, err.Error(), "p2p.disabled requires share.store_ods_only to be enabled")
}

func TestODSOnlyStorage_ConfigApplied(t *testing.T) {
	consNode := core.StartTestNode(t)
	host, port, err := net.SplitHostPort(consNode.GRPCClient.Target())
	require.NoError(t, err)

	cfg := DefaultConfig(node.Bridge)
	cfg.Core.IP = host
	cfg.Core.Port = port
	cfg.Share.StoreODSOnly = true

	require.True(t, cfg.Share.StoreODSOnly, "StoreODSOnly should be true")

	nd := TestNodeWithConfig(t, node.Bridge, cfg)
	require.NotNil(t, nd)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = nd.Start(ctx)
	require.NoError(t, err)

	require.NotNil(t, nd.ShareServ)

	err = nd.Stop(ctx)
	require.NoError(t, err)
}

func TestStorageOnlyMode_OnlyBridgeSupported(t *testing.T) {
	consNode := core.StartTestNode(t)
	host, port, err := net.SplitHostPort(consNode.GRPCClient.Target())
	require.NoError(t, err)

	cfg := DefaultConfig(node.Light)
	cfg.Core.IP = host
	cfg.Core.Port = port
	cfg.P2P.Disabled = true
	cfg.Share.StoreODSOnly = true

	store := MockStore(t, cfg)
	_, err = New(node.Light, modp2p.Private, store)
	require.Error(t, err)
	require.Contains(t, err.Error(), "p2p.disabled is only supported for Bridge nodes")
}
