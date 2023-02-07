package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
)

// Common consts for some of tests producing filled blocks
const (
	blocksAmount = 20                     // number of *filled* *blocks to generate
	blockSize    = 16                     // block size to generate
	blockTime    = time.Millisecond * 300 // how often to generate blocks
)

/*
Test-Case: Sync a Light Node with a Bridge Node(includes DASing of non-empty blocks)
Steps:
1. Create and start a Bridge Node(BN)
2. Check BN is synced to height 20
3. Create and start a Light Node(LN)
4. Connect LN to BN
5. Check LN is synced to height 30
*/
func TestSyncLightWithBridge(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t)
	fillDn := sw.FillBlocks(ctx, blockSize, blocksAmount)
	sw.WaitTillHeight(ctx, 20)

	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 20)
	require.NoError(t, err)
	require.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 20))

	light := sw.NewLightNode()
	err = light.Start(ctx)
	require.NoError(t, err)
	sw.Connect(light.Host.ID(), bridge.Host.ID())

	h, err = light.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)

	err = light.ShareServ.SharesAvailable(ctx, h.DAH)
	assert.NoError(t, err)

	err = light.DASer.WaitCatchUp(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))
	require.NoError(t, <-fillDn)
}

/*
Test-Case: Light Node continues sync after abrupt stop/start
Steps:
1. Create and start a Bridge Node(BN)
2. Check BN is synced to height 20
3. Create and start a Light Node(LN)
4. Connect LN to BN
5. Check LN is synced to height 30
6. Restart LN
7. Check LN is synced to height 40
*/
func TestSyncStartStopLightWithBridge(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t)
	sw.WaitTillHeight(ctx, 50)

	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 20)
	require.NoError(t, err)
	require.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 20))

	light := sw.NewLightNode()
	err = light.Start(ctx)
	require.NoError(t, err)
	sw.Connect(light.Host.ID(), bridge.Host.ID())

	h, err = light.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)
	require.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))

	err = light.Stop(ctx)
	require.NoError(t, err)
	sw.RemoveNode(light, node.Light)

	light = sw.NewLightNode()
	err = light.Start(ctx)
	require.NoError(t, err)
	sw.Connect(light.Host.ID(), bridge.Host.ID())

	h, err = light.HeaderServ.GetByHeight(ctx, 40)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 40))
}

/*
Test-Case: Sync a Full Node with a Bridge Node(includes DASing of non-empty blocks)
Steps:
1. Create and start a Bridge Node(BN)
2. Check BN is synced to height 20
3. Create and start a Full Node(FN)
4. Connect FN to BN
5. Check FN is synced to height 30
*/

func TestSyncFullWithBridge(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(blockTime))
	fillDn := sw.FillBlocks(ctx, blockSize, blocksAmount)
	sw.WaitTillHeight(ctx, 20)

	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 20)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 20))

	full := sw.NewFullNode()
	err = full.Start(ctx)
	require.NoError(t, err)
	sw.Connect(full.Host.ID(), bridge.Host.ID())

	h, err = full.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))

	err = full.ShareServ.SharesAvailable(ctx, h.DAH)
	assert.NoError(t, err)

	err = full.DASer.WaitCatchUp(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))
	require.NoError(t, <-fillDn)
}

/*
Test-Case: Sync a Light Node from a Full Node
Steps:
1. Create and start a Bridge Node(BN)
2. Check BN is synced to height 20
3. Create and start a Full Node(FN)
4. Connect FN to BN
5. Check FN is synced to height 30
6. Create and start a Light Node(LN)
7. Connect LN to FN
8. Check LN is synced to height 50
*/
func TestSyncLightWithFull(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t)
	sw.WaitTillHeight(ctx, 20)

	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 20)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 20))

	full := sw.NewFullNode()
	err = full.Start(ctx)
	require.NoError(t, err)
	sw.Connect(full.Host.ID(), bridge.Host.ID())

	h, err = full.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))

	light := sw.NewLightNode()
	err = light.Start(ctx)
	require.NoError(t, err)

	sw.Disconnect(light.Host.ID(), bridge.Host.ID())
	sw.Connect(light.Host.ID(), full.Host.ID())

	h, err = light.HeaderServ.GetByHeight(ctx, 50)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 50))
}

/*
Test-Case: Sync a Light Node with multiple trusted peers
Steps:
1. Create and start a Bridge Node(BN)
2. Check BN is synced to height 20
3. Create and start a Full Node(FN)
4. Connect FN to BN
5. Check FN is synced to height 30
6. Create and start a Light Node(LN)
7. Connect LN to FN and BN
8. Check LN is synced to height 50
*/
func TestSyncLightFromMultiplePeers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t)
	sw.WaitTillHeight(ctx, 20)

	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 20)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 20))

	full := sw.NewFullNode()
	err = full.Start(ctx)
	require.NoError(t, err)
	sw.Connect(full.Host.ID(), bridge.Host.ID())

	h, err = full.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))

	light := sw.NewLightNode()
	sw.Connect(light.Host.ID(), bridge.Host.ID())
	sw.Connect(light.Host.ID(), full.Host.ID())
	err = light.Start(ctx)
	require.NoError(t, err)

	h, err = light.HeaderServ.GetByHeight(ctx, 50)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 50))
}
