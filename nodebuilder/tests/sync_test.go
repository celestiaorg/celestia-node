package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
)

// Common consts for tests producing filled blocks
const (
	numBlocks = 20
	bsize     = 16
	btime     = time.Millisecond * 300
)

/*
Test-Case: Header and block/sample sync against a Bridge Node of non-empty blocks.

Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Check BN is synced to height 20

Light node:
4. Create a Light Node (LN) with bridge as a trusted peer
5. Start a LN with a defined connection to the BN
6. Check LN is header-synced to height 20
7. Wait until LN has sampled height 20
8. Wait for LN DASer to catch up to network head

Full node:
4. Create a Full Node (FN) with bridge as a trusted peer
5. Start a FN with a defined connection to the BN
6. Check FN is header-synced to height 20
7. Wait until FN has synced block at height 20
8. Wait for FN DASer to catch up to network head
*/
func TestSyncAgainstBridge_NonEmptyChain(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	// wait for core network to fill 20 blocks
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, numBlocks)
	sw.WaitTillHeight(ctx, numBlocks)

	// start a bridge and wait for it to sync to 20
	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.WaitForHeight(ctx, numBlocks)
	require.NoError(t, err)
	require.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, numBlocks))

	t.Run("light sync against bridge", func(t *testing.T) {
		// create a light node that is connected to the bridge node as
		// a bootstrapper
		cfg := nodebuilder.DefaultConfig(node.Light)
		swamp.WithTrustedPeers(t, cfg, bridge)
		light := sw.NewNodeWithConfig(node.Light, cfg)
		// start light node and wait for it to sync 20 blocks
		err = light.Start(ctx)
		require.NoError(t, err)
		h, err = light.HeaderServ.WaitForHeight(ctx, numBlocks)
		require.NoError(t, err)
		assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, numBlocks))

		// check that the light node has also sampled over the block at height 20
		err = light.ShareServ.SharesAvailable(ctx, h.DAH)
		assert.NoError(t, err)

		// wait until the entire chain (up to network head) has been sampled
		err = light.DASer.WaitCatchUp(ctx)
		require.NoError(t, err)
	})

	t.Run("full sync against bridge", func(t *testing.T) {
		// create a full node with bridge node as its bootstrapper
		cfg := nodebuilder.DefaultConfig(node.Full)
		swamp.WithTrustedPeers(t, cfg, bridge)
		full := sw.NewNodeWithConfig(node.Full, cfg)

		// let full node sync 20 blocks
		err = full.Start(ctx)
		require.NoError(t, err)
		h, err = full.HeaderServ.WaitForHeight(ctx, numBlocks)
		require.NoError(t, err)
		assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, numBlocks))

		// check to ensure the full node can sync the 20th block's data
		err = full.ShareServ.SharesAvailable(ctx, h.DAH)
		assert.NoError(t, err)

		// wait for full node to sync up the blocks from genesis -> network head.
		err = full.DASer.WaitCatchUp(ctx)
		require.NoError(t, err)
	})

	// wait for the core block filling process to exit
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case err := <-fillDn:
		require.NoError(t, err)
	}
}

/*
Test-Case: Header and block/sample sync against a Bridge Node of empty blocks.

Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Check BN is synced to height 20

Light node:
4. Create a Light Node (LN) with bridge as a trusted peer
5. Start a LN with a defined connection to the BN
6. Check LN is header-synced to height 20
7. Wait until LN has sampled height 20
8. Wait for LN DASer to catch up to network head

Full node:
4. Create a Full Node (FN) with bridge as a trusted peer
5. Start a FN with a defined connection to the BN
6. Check FN is header-synced to height 20
7. Wait until FN has synced block at height 20
8. Wait for FN DASer to catch up to network head
*/
func TestSyncAgainstBridge_EmptyChain(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	sw.WaitTillHeight(ctx, numBlocks)

	// start a bridge and wait for it to sync to 20
	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)
	h, err := bridge.HeaderServ.WaitForHeight(ctx, numBlocks)
	require.NoError(t, err)
	require.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, numBlocks))

	t.Run("light sync against bridge", func(t *testing.T) {
		// create a light node that is connected to the bridge node as
		// a bootstrapper
		cfg := nodebuilder.DefaultConfig(node.Light)
		swamp.WithTrustedPeers(t, cfg, bridge)
		light := sw.NewNodeWithConfig(node.Light, cfg)
		// start light node and wait for it to sync 20 blocks
		err = light.Start(ctx)
		require.NoError(t, err)
		h, err = light.HeaderServ.WaitForHeight(ctx, numBlocks)
		require.NoError(t, err)
		assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, numBlocks))

		// check that the light node has also sampled over the block at height 20
		err = light.ShareServ.SharesAvailable(ctx, h.DAH)
		assert.NoError(t, err)

		// wait until the entire chain (up to network head) has been sampled
		err = light.DASer.WaitCatchUp(ctx)
		require.NoError(t, err)
	})

	t.Run("full sync against bridge", func(t *testing.T) {
		// create a full node with bridge node as its bootstrapper
		cfg := nodebuilder.DefaultConfig(node.Full)
		swamp.WithTrustedPeers(t, cfg, bridge)
		full := sw.NewNodeWithConfig(node.Full, cfg)

		// let full node sync 20 blocks
		err = full.Start(ctx)
		require.NoError(t, err)
		h, err = full.HeaderServ.WaitForHeight(ctx, numBlocks)
		require.NoError(t, err)
		assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, numBlocks))

		// check to ensure the full node can sync the 20th block's data
		err = full.ShareServ.SharesAvailable(ctx, h.DAH)
		assert.NoError(t, err)

		// wait for full node to sync up the blocks from genesis -> network head.
		err = full.DASer.WaitCatchUp(ctx)
		require.NoError(t, err)
	})
}

/*
Test-Case: Light Node continues sync after abrupt stop/start
Pre-Requisites:
- CoreClient is started by swamp
- CoreClient has generated 50 blocks
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Check BN is synced to height 20
4. Create a Light Node(LN) with a trusted peer
5. Start a LN with a defined connection to the BN
6. Check LN is synced to height 20
7. Disconnect LN from BN for 3 seconds while BN continues broadcasting new blocks from core
8. Re-connect LN and let it sync up again
9. Check LN is synced to height 40
*/
func TestSyncStartStopLightWithBridge(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	defer cancel()

	sw := swamp.NewSwamp(t)
	// wait for core network to fill 20 blocks
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, numBlocks)
	sw.WaitTillHeight(ctx, numBlocks)

	// create bridge
	bridge := sw.NewBridgeNode()
	// and let bridge node sync up with core network
	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.WaitForHeight(ctx, numBlocks)
	require.NoError(t, err)
	require.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, numBlocks))

	// create a light node and connect it to the bridge node as a bootstrapper
	cfg := nodebuilder.DefaultConfig(node.Light)
	swamp.WithTrustedPeers(t, cfg, bridge)
	light := sw.NewNodeWithConfig(node.Light, cfg)

	// start light node and let it sync to 20
	err = light.Start(ctx)
	require.NoError(t, err)
	h, err = light.HeaderServ.WaitForHeight(ctx, numBlocks)
	require.NoError(t, err)
	require.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, numBlocks))

	sw.StopNode(ctx, light)

	cfg = nodebuilder.DefaultConfig(node.Light)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	light = sw.NewNodeWithConfig(node.Light, cfg)
	require.NoError(t, light.Start(ctx))

	// ensure when light node comes back up, it can sync the remainder of the chain it
	// missed while sleeping
	h, err = light.HeaderServ.WaitForHeight(ctx, 40)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 40))

	// wait for the core block filling process to exit
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case err := <-fillDn:
		require.NoError(t, err)
	}
}

/*
Test-Case: Sync a Light Node from a Full Node
Pre-Requisites:
- CoreClient is started by swamp
- CoreClient has generated 20 blocks
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Check BN is synced to height 20
4. Create a Full Node(FN) with a connection to BN as a trusted peer
5. Start a FN
6. Check FN is synced to network head
7. Create a Light Node(LN) with a connection to FN as a trusted peer
8. Ensure LN is NOT connected to BN and only connected to FN
9. Start LN
10. Check LN is synced to network head
*/
func TestSyncLightAgainstFull(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t)
	// wait for the core network to fill up 20 blocks
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, numBlocks)
	sw.WaitTillHeight(ctx, numBlocks)

	// start a bridge node and wait for it to sync up 20 blocks
	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.WaitForHeight(ctx, numBlocks)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, numBlocks))

	// create a FN with BN as a trusted peer
	cfg := nodebuilder.DefaultConfig(node.Full)
	swamp.WithTrustedPeers(t, cfg, bridge)
	full := sw.NewNodeWithConfig(node.Full, cfg)

	// start FN and wait for it to sync up to BN
	err = full.Start(ctx)
	require.NoError(t, err)
	err = full.HeaderServ.SyncWait(ctx)
	require.NoError(t, err)

	// create an LN with FN as a trusted peer
	cfg = nodebuilder.DefaultConfig(node.Light)
	swamp.WithTrustedPeers(t, cfg, full)
	light := sw.NewNodeWithConfig(node.Light, cfg)

	// ensure there is no direct connection between LN and BN so that
	// LN relies only on FN for syncing
	err = sw.Network.UnlinkPeers(bridge.Host.ID(), light.Host.ID())
	require.NoError(t, err)

	// start LN and wait for it to sync up to network head against the FN
	err = light.Start(ctx)
	require.NoError(t, err)
	err = light.HeaderServ.SyncWait(ctx)
	require.NoError(t, err)

	// wait for the core block filling process to exit
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case err := <-fillDn:
		require.NoError(t, err)
	}
}

/*
Test-Case: Sync a Light Node with multiple trusted peers
Pre-Requisites:
- CoreClient is started by swamp
- CoreClient has generated 20 blocks
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Check BN is synced to height 20
4. Create a Full Node(FN) with a connection to BN as a trusted peer
5. Start a FN
6. Check FN is synced to network head
7. Create a Light Node(LN) with a connection to BN and FN as trusted peers
8. Start LN
9. Check LN is synced to network head.
*/
func TestSyncLightWithTrustedPeers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t)
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, numBlocks)
	sw.WaitTillHeight(ctx, numBlocks)

	// create a BN and let it sync to network head
	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)
	err = bridge.HeaderServ.SyncWait(ctx)
	require.NoError(t, err)

	// create a FN with BN as trusted peer
	cfg := nodebuilder.DefaultConfig(node.Full)
	swamp.WithTrustedPeers(t, cfg, bridge)
	full := sw.NewNodeWithConfig(node.Full, cfg)

	// let FN sync to network head
	err = full.Start(ctx)
	require.NoError(t, err)
	err = full.HeaderServ.SyncWait(ctx)
	require.NoError(t, err)

	// create a LN with both FN and BN as trusted peers
	cfg = nodebuilder.DefaultConfig(node.Light)
	swamp.WithTrustedPeers(t, cfg, bridge, full)
	light := sw.NewNodeWithConfig(node.Light, cfg)

	// let LN sync to network head
	err = light.Start(ctx)
	require.NoError(t, err)
	err = light.HeaderServ.SyncWait(ctx)
	require.NoError(t, err)

	// wait for the core block filling process to exit
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case err := <-fillDn:
		require.NoError(t, err)
	}
}
