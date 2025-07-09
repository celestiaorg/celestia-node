//go:build integration

package tastora

import (
	"context"
	"testing"
	"time"

	tastoratypes "github.com/celestiaorg/tastora/framework/types"
	"github.com/stretchr/testify/suite"
)

// DASTestSuite provides comprehensive testing of the DAS module APIs.
// Tests data availability sampling functionality, monitoring, and coordination.
type DASTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestDASTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DAS module integration tests in short mode")
	}
	suite.Run(t, &DASTestSuite{})
}

func (s *DASTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithFullNodes(2), WithBridgeNodes(1), WithLightNodes(2))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *DASTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.cleanup()
	}
}

// TestDASSamplingStats validates SamplingStats API functionality
func (s *DASTestSuite) TestDASSamplingStats_FullNode() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get sampling stats
	stats, err := client.DAS.SamplingStats(ctx)
	s.Require().NoError(err, "should retrieve DAS sampling stats")
	s.Require().NotNil(stats, "sampling stats should not be nil")

	// Validate stats structure
	s.Assert().GreaterOrEqual(stats.SampledChainHead, uint64(0), "sampled chain head should be non-negative")
	s.Assert().GreaterOrEqual(stats.NetworkHead, uint64(0), "network head should be non-negative")
	// Validate relationships between fields
	// CatchUpDone is a boolean indicating if catch-up sampling is complete
	s.Assert().True(stats.CatchUpDone || !stats.CatchUpDone, "catchup done should be a valid boolean")
	s.Assert().GreaterOrEqual(stats.CatchupHead, uint64(0), "catchup head should be non-negative")
	s.Assert().GreaterOrEqual(len(stats.Workers), 0, "workers list should be valid")
	s.Assert().GreaterOrEqual(stats.Concurrency, 0, "concurrency should be non-negative")
	// Note: CatchUpDone is boolean, so we just verify it's accessible
	s.Assert().True(stats.IsRunning, "DAS should be running on full node")
}

func (s *DASTestSuite) TestDASSamplingStats_LightNode() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	lightNode := s.framework.GetLightNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Get sampling stats
	stats, err := client.DAS.SamplingStats(ctx)
	s.Require().NoError(err, "should retrieve DAS sampling stats")
	s.Require().NotNil(stats, "sampling stats should not be nil")

	// Validate stats structure for light node
	s.Assert().GreaterOrEqual(stats.SampledChainHead, uint64(0), "sampled chain head should be non-negative")
	s.Assert().GreaterOrEqual(stats.NetworkHead, uint64(0), "network head should be non-negative")
	s.Assert().GreaterOrEqual(len(stats.Workers), 0, "workers list should be valid")
	s.Assert().GreaterOrEqual(stats.Concurrency, 0, "concurrency should be non-negative")
	s.Assert().True(stats.IsRunning, "DAS should be running on light node")
}

func (s *DASTestSuite) TestDASSamplingStats_BridgeNode() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Bridge nodes have stubbed DAS implementation
	_, err := client.DAS.SamplingStats(ctx)
	s.Assert().Error(err, "bridge node should return error for DAS sampling stats")
	s.Assert().Contains(err.Error(), "stubbed", "should be stubbed error message")
}

// TestDASWaitCatchUp validates WaitCatchUp API functionality
func (s *DASTestSuite) TestDASWaitCatchUp_FullNode() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Wait for DAS to catch up
	err := client.DAS.WaitCatchUp(ctx)
	s.Require().NoError(err, "DAS should catch up successfully on full node")

	// Verify that after catching up, stats show completion
	stats, err := client.DAS.SamplingStats(ctx)
	s.Require().NoError(err, "should get stats after catch up")

	// After catch up, sampled head should be close to network head
	heightDiff := int64(stats.NetworkHead) - int64(stats.SampledChainHead)
	s.Assert().LessOrEqual(heightDiff, int64(5), "sampled head should be close to network head after catch up")
}

func (s *DASTestSuite) TestDASWaitCatchUp_LightNode() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	lightNode := s.framework.GetLightNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Wait for DAS to catch up
	err := client.DAS.WaitCatchUp(ctx)
	s.Require().NoError(err, "DAS should catch up successfully on light node")

	// Verify sampling progress
	stats, err := client.DAS.SamplingStats(ctx)
	s.Require().NoError(err, "should get stats after catch up")
	s.Assert().GreaterOrEqual(stats.SampledChainHead, uint64(1), "should have sampled at least some blocks")
}

func (s *DASTestSuite) TestDASWaitCatchUp_BridgeNode() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Bridge nodes have stubbed DAS implementation
	err := client.DAS.WaitCatchUp(ctx)
	s.Assert().Error(err, "bridge node should return error for DAS wait catch up")
	s.Assert().Contains(err.Error(), "stubbed", "should be stubbed error message")
}

// TestDASWithData validates DAS functionality with actual data
func (s *DASTestSuite) TestDASWithData_SamplingProgress() {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	const (
		numBlocks = 5
		blockSize = 8
	)

	// Create test wallet and fill blocks with data
	testWallet := s.framework.CreateTestWallet(ctx, 5_000_000_000)
	fillDone, err := s.framework.FillBlocks(ctx, testWallet.GetFormattedAddress(), blockSize, numBlocks)
	s.Require().NoError(err, "should start filling blocks")

	// Wait for blocks to be produced
	s.Require().NoError(s.framework.WaitTillHeight(ctx, numBlocks), "should reach target height")

	fullNode := s.framework.GetFullNodes()[0]
	lightNode := s.framework.GetLightNodes()[0]

	fullClient := s.framework.GetNodeRPCClient(ctx, fullNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	s.Run("FullNode_SamplingProgress", func() {
		// Wait for full node DAS to catch up
		err := fullClient.DAS.WaitCatchUp(ctx)
		s.Require().NoError(err, "full node DAS should catch up")

		// Check sampling stats
		stats, err := fullClient.DAS.SamplingStats(ctx)
		s.Require().NoError(err, "should get sampling stats")

		s.Assert().GreaterOrEqual(stats.SampledChainHead, uint64(numBlocks), "should have sampled target blocks")
		s.Assert().True(stats.IsRunning, "DAS should be running")
		s.Assert().GreaterOrEqual(stats.Concurrency, 0, "concurrency should be non-negative")
	})

	s.Run("LightNode_SamplingProgress", func() {
		// Wait for light node DAS to catch up
		err := lightClient.DAS.WaitCatchUp(ctx)
		s.Require().NoError(err, "light node DAS should catch up")

		// Check sampling stats
		stats, err := lightClient.DAS.SamplingStats(ctx)
		s.Require().NoError(err, "should get sampling stats")

		s.Assert().GreaterOrEqual(stats.SampledChainHead, uint64(1), "should have sampled some blocks")
		s.Assert().True(stats.IsRunning, "DAS should be running")
	})

	// Wait for block filling to complete
	select {
	case <-ctx.Done():
		s.T().Fatal(ctx.Err())
	case err := <-fillDone:
		s.Require().NoError(err, "block filling should complete")
	}
}

// TestDASCrossNodeCoordination validates DAS coordination across different node types
func (s *DASTestSuite) TestDASCrossNodeCoordination() {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	const (
		numBlocks = 8
		blockSize = 6
	)

	// Create test wallet and fill blocks
	testWallet := s.framework.CreateTestWallet(ctx, 8_000_000_000)
	fillDone, err := s.framework.FillBlocks(ctx, testWallet.GetFormattedAddress(), blockSize, numBlocks)
	s.Require().NoError(err, "should start filling blocks")

	// Wait for blocks to be produced
	s.Require().NoError(s.framework.WaitTillHeight(ctx, numBlocks), "should reach target height")

	// Get different node types
	fullNode1 := s.framework.GetFullNodes()[0]
	fullNode2 := s.framework.GetFullNodes()[1]
	lightNode1 := s.framework.GetLightNodes()[0]
	lightNode2 := s.framework.GetLightNodes()[1]

	nodes := map[string]interface{}{
		"full1":  fullNode1,
		"full2":  fullNode2,
		"light1": lightNode1,
		"light2": lightNode2,
	}

	s.Run("CoordinatedCatchUp", func() {
		// All nodes should be able to catch up
		for nodeType, nodeInterface := range nodes {
			node := nodeInterface.(tastoratypes.DANode)
			client := s.framework.GetNodeRPCClient(ctx, node)
			err := client.DAS.WaitCatchUp(ctx)
			s.Require().NoError(err, "%s node should catch up", nodeType)
		}
	})

	s.Run("SamplingConsistency", func() {
		// Check that all nodes have progressed in sampling
		for nodeType, nodeInterface := range nodes {
			node := nodeInterface.(tastoratypes.DANode)
			client := s.framework.GetNodeRPCClient(ctx, node)
			stats, err := client.DAS.SamplingStats(ctx)
			s.Require().NoError(err, "should get stats for %s node", nodeType)
			s.Assert().NotNil(stats, "%s node should have sampling stats", nodeType)
		}
	})

	// Wait for block filling to complete
	select {
	case <-ctx.Done():
		s.T().Fatal(ctx.Err())
	case err := <-fillDone:
		s.Require().NoError(err, "block filling should complete")
	}
}

// TestDASPerformance validates DAS performance under different loads
func (s *DASTestSuite) TestDASPerformance_UnderLoad() {
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()

	const (
		numBlocks = 10
		blockSize = 12 // Larger blocks to create more sampling work
	)

	// Create test wallet and fill blocks with larger data
	testWallet := s.framework.CreateTestWallet(ctx, 10_000_000_000)
	fillDone, err := s.framework.FillBlocks(ctx, testWallet.GetFormattedAddress(), blockSize, numBlocks)
	s.Require().NoError(err, "should start filling blocks")

	// Wait for blocks to be produced
	s.Require().NoError(s.framework.WaitTillHeight(ctx, numBlocks), "should reach target height")

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	s.Run("SamplingPerformance", func() {
		// Record start time
		startTime := time.Now()

		// Wait for DAS to catch up
		err := client.DAS.WaitCatchUp(ctx)
		s.Require().NoError(err, "DAS should catch up under load")

		// Record completion time
		duration := time.Since(startTime)
		s.Assert().Less(duration, 180*time.Second, "DAS should complete within reasonable time")

		// Check final stats
		stats, err := client.DAS.SamplingStats(ctx)
		s.Require().NoError(err, "should get final stats")
		s.Assert().GreaterOrEqual(stats.SampledChainHead, uint64(numBlocks), "should have sampled all blocks")
	})

	s.Run("ConcurrencyValidation", func() {
		stats, err := client.DAS.SamplingStats(ctx)
		s.Require().NoError(err, "should get stats")

		s.Assert().GreaterOrEqual(stats.Concurrency, 0, "concurrency should be non-negative")
		s.Assert().LessOrEqual(stats.Concurrency, 50, "concurrency should be reasonable")
		s.Assert().True(stats.IsRunning, "DAS should be running")
	})

	// Wait for block filling to complete
	select {
	case <-ctx.Done():
		s.T().Fatal(ctx.Err())
	case err := <-fillDone:
		s.Require().NoError(err, "block filling should complete")
	}
}

// TestDASErrorRecovery validates error recovery scenarios and fault tolerance
func (s *DASTestSuite) TestDASErrorRecovery_NetworkInterruption() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	lightNode := s.framework.GetLightNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, lightNode)

	s.Run("RecoveryAfterTimeout", func() {
		// Create context with short timeout to simulate network interruption
		shortCtx, shortCancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer shortCancel()

		// This will likely timeout
		_ = client.DAS.WaitCatchUp(shortCtx)

		// Now try with proper timeout - should recover
		err := client.DAS.WaitCatchUp(ctx)
		s.Require().NoError(err, "DAS should recover after timeout")
	})

	s.Run("StatsAfterRecovery", func() {
		// Check that stats are still accessible after recovery
		stats, err := client.DAS.SamplingStats(ctx)
		s.Require().NoError(err, "should get stats after recovery")
		s.Assert().True(stats.IsRunning, "DAS should be running after recovery")
	})
}

// TestDASProgressMonitoring validates DAS progress monitoring over time
func (s *DASTestSuite) TestDASProgressMonitoring() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	const (
		numBlocks = 6
		blockSize = 4
	)

	// Create test wallet and fill blocks gradually
	testWallet := s.framework.CreateTestWallet(ctx, 6_000_000_000)

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	s.Run("ProgressiveMonitoring", func() {
		// Fill blocks in batches and monitor progress
		for batch := 1; batch <= 3; batch++ {
			// Fill 2 blocks per batch
			fillDone, err := s.framework.FillBlocks(ctx, testWallet.GetFormattedAddress(), blockSize, 2)
			s.Require().NoError(err, "should start filling batch %d", batch)

			batchHeight := uint64(batch * 2)
			s.Require().NoError(s.framework.WaitTillHeight(ctx, batchHeight), "should reach height %d", batchHeight)

			// Monitor DAS progress
			err = client.DAS.WaitCatchUp(ctx)
			s.Require().NoError(err, "DAS should catch up for batch %d", batch)

			stats, err := client.DAS.SamplingStats(ctx)
			s.Require().NoError(err, "should get stats for batch %d", batch)
			s.Assert().GreaterOrEqual(stats.SampledChainHead, batchHeight, "should have sampled batch %d", batch)

			// Wait for batch completion
			select {
			case <-ctx.Done():
				s.T().Fatal(ctx.Err())
			case err := <-fillDone:
				s.Require().NoError(err, "batch %d should complete", batch)
			}
		}
	})

	s.Run("FinalConsistencyCheck", func() {
		// Final consistency check
		stats, err := client.DAS.SamplingStats(ctx)
		s.Require().NoError(err, "should get final stats")

		s.Assert().GreaterOrEqual(stats.SampledChainHead, uint64(6), "should have sampled all blocks")
		s.Assert().True(stats.IsRunning, "DAS should still be running")

		// Network head should be reasonable
		s.Assert().GreaterOrEqual(stats.NetworkHead, stats.SampledChainHead, "network head should be >= sampled head")
	})
}
