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
	// Start a test core node
	consNode := core.StartTestNode(t)
	host, port, err := net.SplitHostPort(consNode.GRPCClient.Target())
	require.NoError(t, err)

	// Create config with P2P disabled and ODS-only enabled
	cfg := DefaultConfig(node.Bridge)
	cfg.Core.IP = host
	cfg.Core.Port = port
	cfg.P2P.Disabled = true
	cfg.Share.StoreODSOnly = true

	// Create node with storage-only mode
	nd := TestNodeWithConfig(t, node.Bridge, cfg)
	require.NotNil(t, nd)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the node
	err = nd.Start(ctx)
	require.NoError(t, err)

	// Verify P2P components are nil
	require.Nil(t, nd.Host, "P2P Host should be nil when P2P is disabled")
	require.Nil(t, nd.ConnGater, "ConnGater should be nil when P2P is disabled")
	require.Nil(t, nd.Routing, "Routing should be nil when P2P is disabled")
	require.Nil(t, nd.DataExchange, "DataExchange should be nil when P2P is disabled")
	require.Nil(t, nd.BlockService, "BlockService should be nil when P2P is disabled")
	require.Nil(t, nd.PubSub, "PubSub should be nil when P2P is disabled")

	// Verify core components are still available
	require.NotNil(t, nd.RPCServer, "RPC Server should be available")
	require.NotNil(t, nd.ShareServ, "Share service should be available")
	require.NotNil(t, nd.HeaderServ, "Header service should be available")
	require.NotNil(t, nd.StateServ, "State service should be available")

	// Stop the node
	err = nd.Stop(ctx)
	require.NoError(t, err)
}

func TestStorageOnlyMode_ValidationFailure(t *testing.T) {
	consNode := core.StartTestNode(t)
	host, port, err := net.SplitHostPort(consNode.GRPCClient.Target())
	require.NoError(t, err)

	// Create config with P2P disabled but ODS-only NOT enabled (should fail)
	cfg := DefaultConfig(node.Bridge)
	cfg.Core.IP = host
	cfg.Core.Port = port
	cfg.P2P.Disabled = true
	cfg.Share.StoreODSOnly = false // This should cause validation to fail

	// Attempt to create node - should fail during module construction
	store := MockStore(t, cfg)
	_, err = New(node.Bridge, modp2p.Private, store)
	require.Error(t, err)
	require.Contains(t, err.Error(), "p2p.disabled requires share.store_ods_only to be enabled")
}

func TestODSOnlyStorage_ConfigApplied(t *testing.T) {
	consNode := core.StartTestNode(t)
	host, port, err := net.SplitHostPort(consNode.GRPCClient.Target())
	require.NoError(t, err)

	// Create config with ODS-only enabled
	cfg := DefaultConfig(node.Bridge)
	cfg.Core.IP = host
	cfg.Core.Port = port
	cfg.Share.StoreODSOnly = true

	// Verify config is set correctly
	require.True(t, cfg.Share.StoreODSOnly, "StoreODSOnly should be true")

	// Create node - should succeed
	nd := TestNodeWithConfig(t, node.Bridge, cfg)
	require.NotNil(t, nd)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = nd.Start(ctx)
	require.NoError(t, err)

	// Node should start successfully with ODS-only config
	require.NotNil(t, nd.ShareServ)

	err = nd.Stop(ctx)
	require.NoError(t, err)
}

func TestStorageOnlyMode_OnlyBridgeSupported(t *testing.T) {
	consNode := core.StartTestNode(t)
	host, port, err := net.SplitHostPort(consNode.GRPCClient.Target())
	require.NoError(t, err)

	// Try to create Light node with P2P disabled - should fail
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
