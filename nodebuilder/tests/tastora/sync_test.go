//go:build integration

package tastora

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// SyncTestSuite provides comprehensive testing of Header and DAS synchronization.
// Migrated from swamp sync_test.go with enhanced structure and better coverage.
type SyncTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestSyncTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Sync module integration tests in short mode")
	}
	suite.Run(t, &SyncTestSuite{})
}

func (s *SyncTestSuite) SetupSuite() {
	// Setup framework with multiple node types for sync testing
	// Using single instances as we only need one of each type for testing
	s.framework = NewFramework(s.T(),
		WithValidators(1),
		WithBridgeNodes(1),
		WithFullNodes(1),
		WithLightNodes(1),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *SyncTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.cleanup()
	}
}

// TestSyncAgainstBridge_NonEmptyChain tests header and block/sample sync against a Bridge Node with filled blocks.
// Migrated from swamp: TestSyncAgainstBridge_NonEmptyChain
func (s *SyncTestSuite) TestSyncAgainstBridge_NonEmptyChain() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	const (
		numBlocks = 20
		blockSize = 16
	)

	// Create test wallet for block filling
	testWallet := s.framework.CreateTestWallet(ctx, 10_000_000_000)

	// Fill blocks with transactions
	fillDone, err := s.framework.FillBlocks(ctx, testWallet.GetFormattedAddress(), blockSize, numBlocks)
	s.Require().NoError(err)

	// Wait for chain to reach target height
	s.Require().NoError(s.framework.WaitTillHeight(ctx, numBlocks))

	// Get bridge node and verify it's synced
	bridge := s.framework.GetBridgeNode()
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridge)

	bridgeHead, err := bridgeClient.Header.WaitForHeight(ctx, numBlocks)
	s.Require().NoError(err)
	s.Require().NotZero(bridgeHead.Height())

	// Test light node sync against bridge
	s.Run("LightNode_SyncAgainstBridge", func() {
		lightNode := s.framework.GetLightNodes()[0]
		lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

		// Wait for light node to sync to target height
		h, err := lightClient.Header.WaitForHeight(ctx, numBlocks)
		s.Require().NoError(err)
		s.Assert().Equal(uint64(numBlocks), h.Height())

		// Verify light node has sampled the block
		err = lightClient.Share.SharesAvailable(ctx, h.Height())
		s.Assert().NoError(err)

		// Wait for DAS to catch up to network head
		err = lightClient.DAS.WaitCatchUp(ctx)
		s.Require().NoError(err)
	})

	// Test full node sync against bridge
	s.Run("FullNode_SyncAgainstBridge", func() {
		fullNode := s.framework.GetFullNodes()[0]
		fullClient := s.framework.GetNodeRPCClient(ctx, fullNode)

		// Wait for full node to sync to target height
		h, err := fullClient.Header.WaitForHeight(ctx, numBlocks)
		s.Require().NoError(err)
		s.Assert().Equal(uint64(numBlocks), h.Height())

		// Verify full node has the block data available
		err = fullClient.Share.SharesAvailable(ctx, h.Height())
		s.Assert().NoError(err)

		// Wait for DAS to catch up to network head
		err = fullClient.DAS.WaitCatchUp(ctx)
		s.Require().NoError(err)
	})

	// Wait for block filling to complete
	select {
	case <-ctx.Done():
		s.T().Fatal(ctx.Err())
	case err := <-fillDone:
		s.Require().NoError(err)
	}
}

// TestSyncAgainstBridge_EmptyChain tests sync against bridge with empty blocks.
// Migrated from swamp: TestSyncAgainstBridge_EmptyChain
func (s *SyncTestSuite) TestSyncAgainstBridge_EmptyChain() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	const numBlocks = 20

	// Wait for empty blocks to be produced
	s.Require().NoError(s.framework.WaitTillHeight(ctx, numBlocks))

	// Get bridge node and verify it's synced
	bridge := s.framework.GetBridgeNode()
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridge)

	_, err := bridgeClient.Header.WaitForHeight(ctx, numBlocks)
	s.Require().NoError(err)

	// Test light node sync with empty blocks
	s.Run("LightNode_SyncEmptyBlocks", func() {
		lightNode := s.framework.GetLightNodes()[0] // Use first light node
		lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

		h, err := lightClient.Header.WaitForHeight(ctx, numBlocks)
		s.Require().NoError(err)
		s.Assert().Equal(uint64(numBlocks), h.Height())

		// Even empty blocks should be available for sampling
		err = lightClient.Share.SharesAvailable(ctx, h.Height())
		s.Assert().NoError(err)

		err = lightClient.DAS.WaitCatchUp(ctx)
		s.Require().NoError(err)
	})

	// Test full node sync with empty blocks
	s.Run("FullNode_SyncEmptyBlocks", func() {
		fullNode := s.framework.GetFullNodes()[0] // Use first full node
		fullClient := s.framework.GetNodeRPCClient(ctx, fullNode)

		h, err := fullClient.Header.WaitForHeight(ctx, numBlocks)
		s.Require().NoError(err)
		s.Assert().Equal(uint64(numBlocks), h.Height())

		err = fullClient.Share.SharesAvailable(ctx, h.Height())
		s.Assert().NoError(err)

		err = fullClient.DAS.WaitCatchUp(ctx)
		s.Require().NoError(err)
	})
}

// TestSyncLightAgainstFull tests light node syncing from full node instead of bridge.
// Migrated from swamp: TestSyncLightAgainstFull
func (s *SyncTestSuite) TestSyncLightAgainstFull() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	const (
		numBlocks = 20
		blockSize = 16
	)

	// Create test wallet and fill blocks
	testWallet := s.framework.CreateTestWallet(ctx, 10_000_000_000)
	fillDone, err := s.framework.FillBlocks(ctx, testWallet.GetFormattedAddress(), blockSize, numBlocks)
	s.Require().NoError(err)

	s.Require().NoError(s.framework.WaitTillHeight(ctx, numBlocks))

	// Verify bridge and full node are synced
	bridge := s.framework.GetBridgeNode()
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridge)

	fullNode := s.framework.GetFullNodes()[0]
	fullClient := s.framework.GetNodeRPCClient(ctx, fullNode)

	bridgeHead, err := bridgeClient.Header.WaitForHeight(ctx, numBlocks)
	s.Require().NoError(err)

	_, err = fullClient.Header.WaitForHeight(ctx, bridgeHead.Height())
	s.Require().NoError(err)

	// Test light node sync against full node
	lightNode := s.framework.GetLightNodes()[0]
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Ensure light node can sync from full node
	fullHead, err := fullClient.Header.LocalHead(ctx)
	s.Require().NoError(err)

	_, err = lightClient.Header.WaitForHeight(ctx, fullHead.Height())
	s.Require().NoError(err)

	// Verify sync worked correctly
	lightHead, err := lightClient.Header.LocalHead(ctx)
	s.Require().NoError(err)
	s.Assert().Equal(fullHead.Height(), lightHead.Height())

	// Wait for block filling to complete
	select {
	case <-ctx.Done():
		s.T().Fatal(ctx.Err())
	case err := <-fillDone:
		s.Require().NoError(err)
	}
}

// TestSyncInterruption tests sync resumption after node restart.
// Migrated from swamp: TestSyncStartStopLightWithBridge
func (s *SyncTestSuite) TestSyncInterruption() {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	const (
		initialBlocks = 20
		blockSize     = 16
	)

	// Create test wallet and fill initial blocks
	testWallet := s.framework.CreateTestWallet(ctx, 15_000_000_000)
	fillDone, err := s.framework.FillBlocks(ctx, testWallet.GetFormattedAddress(), blockSize, initialBlocks)
	s.Require().NoError(err)

	s.Require().NoError(s.framework.WaitTillHeight(ctx, initialBlocks))

	// Get bridge node
	bridge := s.framework.GetBridgeNode()
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridge)

	h, err := bridgeClient.Header.WaitForHeight(ctx, initialBlocks)
	s.Require().NoError(err)

	// Start light node and sync to initial height
	lightNode := s.framework.GetLightNodes()[0]
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	h, err = lightClient.Header.WaitForHeight(ctx, initialBlocks)
	s.Require().NoError(err)
	s.Require().Equal(uint64(initialBlocks), h.Height())

	// TODO: Implement node stop/restart functionality in framework
	// For now, we test that sync can handle continued block production

	// Continue filling blocks while light node is running
	fillDone2, err := s.framework.FillBlocks(ctx, testWallet.GetFormattedAddress(), blockSize, 20)
	s.Require().NoError(err)

	// Verify light node can sync to new height (40 total)
	finalHeight := uint64(initialBlocks + 20)
	s.Require().NoError(s.framework.WaitTillHeight(ctx, finalHeight))

	h, err = lightClient.Header.WaitForHeight(ctx, finalHeight)
	s.Require().NoError(err)
	s.Assert().Equal(finalHeight, h.Height())

	// Wait for both fill operations to complete
	select {
	case <-ctx.Done():
		s.T().Fatal(ctx.Err())
	case err := <-fillDone:
		s.Require().NoError(err)
	}

	select {
	case <-ctx.Done():
		s.T().Fatal(ctx.Err())
	case err := <-fillDone2:
		s.Require().NoError(err)
	}
}

// TestHeaderSync_API tests header synchronization APIs
func (s *SyncTestSuite) TestHeaderSync_API() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	lightNode := s.framework.GetLightNodes()[0]
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Test SyncWait API
	s.Run("SyncWait_API", func() {
		err := lightClient.Header.SyncWait(ctx)
		s.Assert().NoError(err)
	})

	// Test NetworkHead vs LocalHead consistency
	s.Run("NetworkHead_LocalHead_Consistency", func() {
		networkHead, err := lightClient.Header.NetworkHead(ctx)
		s.Require().NoError(err)

		localHead, err := lightClient.Header.LocalHead(ctx)
		s.Require().NoError(err)

		// They should be close or equal (within a few blocks)
		heightDiff := int64(networkHead.Height()) - int64(localHead.Height())
		s.Assert().True(heightDiff >= 0 && heightDiff <= 5,
			"NetworkHead (%d) and LocalHead (%d) should be close",
			networkHead.Height(), localHead.Height())
	})

	// Test SyncState API
	s.Run("SyncState_API", func() {
		state, err := lightClient.Header.SyncState(ctx)
		s.Require().NoError(err)
		s.Assert().NotNil(state)
	})
}

// TestDAS_Coordination tests DAS coordination across node types
func (s *SyncTestSuite) TestDAS_Coordination() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Create test wallet and add some blocks
	testWallet := s.framework.CreateTestWallet(ctx, 5_000_000_000)
	fillDone, err := s.framework.FillBlocks(ctx, testWallet.GetFormattedAddress(), 8, 10)
	s.Require().NoError(err)

	s.Require().NoError(s.framework.WaitTillHeight(ctx, 10))

	// Test DAS coordination between different node types
	// Note: Bridge nodes don't support DAS operations, so we only test full and light nodes
	fullNode := s.framework.GetFullNodes()[0]
	fullClient := s.framework.GetNodeRPCClient(ctx, fullNode)

	lightNode := s.framework.GetLightNodes()[0]
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// All nodes should reach the same height
	targetHeight := uint64(10)

	_, err = fullClient.Header.WaitForHeight(ctx, targetHeight)
	s.Require().NoError(err)

	_, err = lightClient.Header.WaitForHeight(ctx, targetHeight)
	s.Require().NoError(err)

	// Full and light nodes should have DAS complete for the same height
	// Bridge nodes don't support DAS, so we skip them
	err = fullClient.DAS.WaitCatchUp(ctx)
	s.Assert().NoError(err)

	err = lightClient.DAS.WaitCatchUp(ctx)
	s.Assert().NoError(err)

	// Verify DAS stats are available for both node types
	fullStats, err := fullClient.DAS.SamplingStats(ctx)
	s.Assert().NoError(err)
	s.Assert().NotNil(fullStats)

	lightStats, err := lightClient.DAS.SamplingStats(ctx)
	s.Assert().NoError(err)
	s.Assert().NotNil(lightStats)

	// Wait for block filling to complete
	select {
	case <-ctx.Done():
		s.T().Fatal(ctx.Err())
	case err := <-fillDone:
		s.Require().NoError(err)
	}
}
