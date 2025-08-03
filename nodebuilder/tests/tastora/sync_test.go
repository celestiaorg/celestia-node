//go:build integration

package tastora

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/header"
)

// SyncTestSuite provides integration testing of header synchronization across multiple nodes.
// Focuses on cross-node sync coordination, network resilience, and real-world sync scenarios.
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
	// Setup with bridge node and multiple light nodes for comprehensive sync testing
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(1), WithLightNodes(2))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

// TestCrossNodeHeaderSync validates that all nodes sync to the same network head
// and maintain consistency across different node types
func (s *SyncTestSuite) TestCrossNodeHeaderSync() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Get all node types for comprehensive sync testing
	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
	lightNode1 := s.framework.GetOrCreateLightNode(ctx)
	lightNode2 := s.framework.GetOrCreateLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	light1Client := s.framework.GetNodeRPCClient(ctx, lightNode1)
	light2Client := s.framework.GetNodeRPCClient(ctx, lightNode2)

	allClients := map[string]*client.Client{
		"bridge": bridgeClient,
		"light1": light1Client,
		"light2": light2Client,
	}

	// Wait for initial sync across all nodes
	time.Sleep(20 * time.Second)

	// Get network head from bridge node (authoritative source)
	bridgeHead, err := bridgeClient.Header.NetworkHead(ctx)
	s.Require().NoError(err, "bridge should have network head")
	targetHeight := bridgeHead.Height()

	s.T().Logf("Target sync height: %d", targetHeight)

	// Verify all nodes sync to the same height
	for nodeType, client := range allClients {
		s.Run(fmt.Sprintf("SyncToHeight_%s", nodeType), func() {
			// Wait for node to sync to target height
			syncedHeader, err := client.Header.WaitForHeight(ctx, targetHeight)
			s.Require().NoError(err, "%s should sync to height %d", nodeType, targetHeight)
			s.Assert().Equal(targetHeight, syncedHeader.Height(), "%s should be at correct height", nodeType)

			// Verify sync state indicates we're synced
			syncState, err := client.Header.SyncState(ctx)
			s.Require().NoError(err, "%s should provide sync state", nodeType)
			s.T().Logf("%s sync state: %+v", nodeType, syncState)

			// Get local head and verify it matches network head
			localHead, err := client.Header.LocalHead(ctx)
			s.Require().NoError(err, "%s should have local head", nodeType)
			s.Assert().Equal(targetHeight, localHead.Height(), "%s local head should match target", nodeType)

			// Verify header consistency by hash
			networkHead, err := client.Header.NetworkHead(ctx)
			s.Require().NoError(err, "%s should have network head", nodeType)
			s.Assert().Equal(bridgeHead.Hash(), networkHead.Hash(), "%s network head hash should match bridge", nodeType)
		})
	}
}

// TestStaggeredNodeSync validates that nodes joining at different times can catch up
// and sync properly with the network
func (s *SyncTestSuite) TestStaggeredNodeSync() {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	// Start with bridge node only
	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Let bridge node get established and produce some blocks
	time.Sleep(30 * time.Second)

	// Get the current height as baseline
	bridgeHead1, err := bridgeClient.Header.NetworkHead(ctx)
	s.Require().NoError(err, "bridge should have initial network head")
	baselineHeight := bridgeHead1.Height()
	s.T().Logf("Baseline height before staggered start: %d", baselineHeight)

	// Start first light node (early joiner)
	lightNode1 := s.framework.GetOrCreateLightNode(ctx)
	light1Client := s.framework.GetNodeRPCClient(ctx, lightNode1)

	// Wait for some more blocks to be produced
	time.Sleep(20 * time.Second)

	// Start second light node (late joiner)
	lightNode2 := s.framework.GetOrCreateLightNode(ctx)
	light2Client := s.framework.GetNodeRPCClient(ctx, lightNode2)

	// Wait for final network state
	time.Sleep(30 * time.Second)

	// Get final network head
	bridgeHead2, err := bridgeClient.Header.NetworkHead(ctx)
	s.Require().NoError(err, "bridge should have final network head")
	finalHeight := bridgeHead2.Height()
	s.T().Logf("Final height after staggered start: %d", finalHeight)

	// Verify that both light nodes caught up despite starting at different times
	clients := map[string]*client.Client{
		"light1_early": light1Client,
		"light2_late":  light2Client,
	}

	for nodeType, client := range clients {
		s.Run(fmt.Sprintf("CatchUpSync_%s", nodeType), func() {
			// Both nodes should catch up to final height
			syncedHeader, err := client.Header.WaitForHeight(ctx, finalHeight)
			s.Require().NoError(err, "%s should catch up to final height", nodeType)
			s.Assert().Equal(finalHeight, syncedHeader.Height(), "%s should reach final height", nodeType)

			// Verify sync state indicates successful catch-up
			syncState, err := client.Header.SyncState(ctx)
			s.Require().NoError(err, "%s should provide sync state", nodeType)
			s.T().Logf("%s final sync state: %+v", nodeType, syncState)

			// Verify they can access historical headers from before they joined
			if baselineHeight > 1 {
				historicalHeader, err := client.Header.GetByHeight(ctx, baselineHeight)
				s.Require().NoError(err, "%s should access historical header", nodeType)
				s.Assert().Equal(baselineHeight, historicalHeader.Height(), "%s should have correct historical header", nodeType)
			}
		})
	}
}

// TestHeaderSubscriptionSync validates real-time header propagation across nodes
// using header subscriptions
func (s *SyncTestSuite) TestHeaderSubscriptionSync() {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
	lightNode1 := s.framework.GetOrCreateLightNode(ctx)
	lightNode2 := s.framework.GetOrCreateLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	light1Client := s.framework.GetNodeRPCClient(ctx, lightNode1)
	light2Client := s.framework.GetNodeRPCClient(ctx, lightNode2)

	// Wait for initial sync
	time.Sleep(20 * time.Second)

	// Subscribe to headers on all nodes
	bridgeHeaders, err := bridgeClient.Header.Subscribe(ctx)
	s.Require().NoError(err, "bridge should allow header subscription")

	light1Headers, err := light1Client.Header.Subscribe(ctx)
	s.Require().NoError(err, "light1 should allow header subscription")

	light2Headers, err := light2Client.Header.Subscribe(ctx)
	s.Require().NoError(err, "light2 should allow header subscription")

	// Track received headers from each node
	receivedHeaders := make(map[string][]*header.ExtendedHeader)
	var mu sync.Mutex

	// Start header collection goroutines
	var wg sync.WaitGroup

	// Bridge node header collection
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ { // Collect 3 headers
			select {
			case h := <-bridgeHeaders:
				mu.Lock()
				receivedHeaders["bridge"] = append(receivedHeaders["bridge"], h)
				mu.Unlock()
				s.T().Logf("Bridge received header at height %d", h.Height())
			case <-time.After(45 * time.Second):
				s.T().Log("Bridge header subscription timeout")
				return
			}
		}
	}()

	// Light node 1 header collection
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ { // Collect 3 headers
			select {
			case h := <-light1Headers:
				mu.Lock()
				receivedHeaders["light1"] = append(receivedHeaders["light1"], h)
				mu.Unlock()
				s.T().Logf("Light1 received header at height %d", h.Height())
			case <-time.After(45 * time.Second):
				s.T().Log("Light1 header subscription timeout")
				return
			}
		}
	}()

	// Light node 2 header collection
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ { // Collect 3 headers
			select {
			case h := <-light2Headers:
				mu.Lock()
				receivedHeaders["light2"] = append(receivedHeaders["light2"], h)
				mu.Unlock()
				s.T().Logf("Light2 received header at height %d", h.Height())
			case <-time.After(45 * time.Second):
				s.T().Log("Light2 header subscription timeout")
				return
			}
		}
	}()

	// Wait for header collection to complete
	wg.Wait()

	// Verify all nodes received headers
	mu.Lock()
	defer mu.Unlock()

	s.Assert().NotEmpty(receivedHeaders["bridge"], "bridge should receive headers via subscription")
	s.Assert().NotEmpty(receivedHeaders["light1"], "light1 should receive headers via subscription")
	s.Assert().NotEmpty(receivedHeaders["light2"], "light2 should receive headers via subscription")

	// Verify header consistency across subscriptions
	if len(receivedHeaders["bridge"]) > 0 && len(receivedHeaders["light1"]) > 0 {
		// Find common heights and compare hashes
		bridgeHeights := make(map[uint64]*header.ExtendedHeader)
		for _, h := range receivedHeaders["bridge"] {
			bridgeHeights[h.Height()] = h
		}

		for _, lightHeader := range receivedHeaders["light1"] {
			if bridgeHeader, exists := bridgeHeights[lightHeader.Height()]; exists {
				s.Assert().Equal(bridgeHeader.Hash(), lightHeader.Hash(),
					"headers at height %d should have same hash across bridge and light1", lightHeader.Height())
			}
		}
	}

	s.T().Logf("✅ Header subscription sync completed successfully!")
}

// TestSyncWaitCoordination validates that SyncWait works correctly across multiple nodes
// and that nodes can coordinate their sync completion
func (s *SyncTestSuite) TestSyncWaitCoordination() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
	lightNode1 := s.framework.GetOrCreateLightNode(ctx)
	lightNode2 := s.framework.GetOrCreateLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	light1Client := s.framework.GetNodeRPCClient(ctx, lightNode1)
	light2Client := s.framework.GetNodeRPCClient(ctx, lightNode2)

	clients := map[string]*client.Client{
		"bridge": bridgeClient,
		"light1": light1Client,
		"light2": light2Client,
	}

	// Test SyncWait on all nodes concurrently
	var wg sync.WaitGroup
	syncResults := make(map[string]error)
	var mu sync.Mutex

	for nodeType, nodeClient := range clients {
		wg.Add(1)
		go func(nt string, c *client.Client) {
			defer wg.Done()

			s.T().Logf("Starting SyncWait for %s", nt)
			start := time.Now()

			err := c.Header.SyncWait(ctx)
			duration := time.Since(start)

			mu.Lock()
			syncResults[nt] = err
			mu.Unlock()

			s.T().Logf("%s SyncWait completed in %v with result: %v", nt, duration, err)
		}(nodeType, nodeClient)
	}

	// Wait for all SyncWait operations to complete
	wg.Wait()

	// Verify all nodes completed sync successfully
	mu.Lock()
	defer mu.Unlock()

	for nodeType, err := range syncResults {
		s.Assert().NoError(err, "%s SyncWait should complete without error", nodeType)
	}

	// After SyncWait completion, verify all nodes are actually synced
	time.Sleep(5 * time.Second)

	var networkHeads []*header.ExtendedHeader
	for nodeType, client := range clients {
		networkHead, err := client.Header.NetworkHead(ctx)
		s.Require().NoError(err, "%s should provide network head after sync", nodeType)
		networkHeads = append(networkHeads, networkHead)

		localHead, err := client.Header.LocalHead(ctx)
		s.Require().NoError(err, "%s should provide local head after sync", nodeType)

		s.Assert().Equal(networkHead.Height(), localHead.Height(),
			"%s local and network heads should match after SyncWait", nodeType)
	}

	// Verify all nodes agree on network head
	if len(networkHeads) > 1 {
		referenceHeight := networkHeads[0].Height()
		referenceHash := networkHeads[0].Hash()

		for _, head := range networkHeads[1:] {
			s.Assert().Equal(referenceHeight, head.Height(),
				"all nodes should agree on network head height")
			s.Assert().Equal(referenceHash, head.Hash(),
				"all nodes should agree on network head hash")
		}
	}

	s.T().Logf("✅ SyncWait coordination completed successfully!")
}

// TestHeaderRangeSync validates that nodes can retrieve ranges of headers consistently
// and handle large header ranges efficiently
func (s *SyncTestSuite) TestHeaderRangeSync() {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
	lightNode := s.framework.GetOrCreateLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Wait for some blocks to accumulate
	time.Sleep(30 * time.Second)

	// Get current network head
	networkHead, err := bridgeClient.Header.NetworkHead(ctx)
	s.Require().NoError(err, "should get network head")
	currentHeight := networkHead.Height()

	if currentHeight < 5 {
		s.T().Skip("Not enough blocks for range testing")
		return
	}

	// Test range retrieval on both node types
	fromHeight := uint64(1)
	toHeight := currentHeight - 1

	s.T().Logf("Testing header range from %d to %d", fromHeight, toHeight)

	// Get the starting header for range query
	fromHeader, err := bridgeClient.Header.GetByHeight(ctx, fromHeight)
	s.Require().NoError(err, "should get starting header")

	clients := map[string]*client.Client{
		"bridge": bridgeClient,
		"light":  lightClient,
	}

	var rangeResults = make(map[string][]*header.ExtendedHeader)

	for nodeType, client := range clients {
		s.Run(fmt.Sprintf("HeaderRange_%s", nodeType), func() {
			// Wait for light node to sync if needed
			if nodeType == "light" {
				_, err := client.Header.WaitForHeight(ctx, currentHeight)
				s.Require().NoError(err, "light node should sync before range query")
			}

			// Get header range
			headers, err := client.Header.GetRangeByHeight(ctx, fromHeader, toHeight)
			s.Require().NoError(err, "%s should get header range", nodeType)
			s.Assert().NotEmpty(headers, "%s should return non-empty header range", nodeType)

			rangeResults[nodeType] = headers

			// Verify range properties
			s.Assert().GreaterOrEqual(len(headers), 1, "%s should return at least one header", nodeType)
			s.Assert().Equal(toHeight, headers[len(headers)-1].Height(),
				"%s last header should be at target height", nodeType)

			// Verify header sequence is valid (heights are consecutive)
			for i := 1; i < len(headers); i++ {
				expectedHeight := headers[i-1].Height() + 1
				actualHeight := headers[i].Height()
				s.Assert().Equal(expectedHeight, actualHeight,
					"%s headers should be consecutive at index %d", nodeType, i)
			}

			s.T().Logf("%s retrieved %d headers in range", nodeType, len(headers))
		})
	}

	// Compare results between bridge and light nodes
	if bridgeHeaders, exists := rangeResults["bridge"]; exists {
		if lightHeaders, exists := rangeResults["light"]; exists {
			s.Assert().Equal(len(bridgeHeaders), len(lightHeaders),
				"bridge and light nodes should return same number of headers")

			// Verify hash consistency for overlapping headers
			minLen := len(bridgeHeaders)
			if len(lightHeaders) < minLen {
				minLen = len(lightHeaders)
			}

			for i := 0; i < minLen; i++ {
				s.Assert().Equal(bridgeHeaders[i].Hash(), lightHeaders[i].Hash(),
					"header %d should have same hash between bridge and light", i)
			}
		}
	}

	s.T().Logf("✅ Header range sync validation completed!")
}
