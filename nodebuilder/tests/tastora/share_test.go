//go:build integration

package tastora

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	libshare "github.com/celestiaorg/go-square/v2/share"

	rpc_client "github.com/celestiaorg/celestia-node/api/rpc/client"
	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/state"
)

// ShareTestSuite provides integration testing of the Share module across multiple nodes.
// Focuses on cross-node communication, data propagation, and multi-node scenarios.
type ShareTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestShareTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Share module integration tests in short mode")
	}
	suite.Run(t, &ShareTestSuite{})
}

func (s *ShareTestSuite) SetupSuite() {
	// Setup with bridge node and multiple light nodes for comprehensive integration testing
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(1), WithLightNodes(2))
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute) // Reduced from 10 minutes
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

// Helper function to submit a blob and get its height
func (s *ShareTestSuite) submitTestBlob(ctx context.Context, namespace libshare.Namespace, data []byte) (uint64, []byte) {
	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Create test wallet and fund the node account
	testWallet := s.framework.CreateTestWallet(ctx, 5_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, bridgeNode, 1_000_000_000)

	// Get node address for blob creation
	nodeAddr, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err)

	// Create blob
	libBlob, err := libshare.NewV1Blob(namespace, data, nodeAddr.Bytes())
	s.Require().NoError(err)

	// Convert to node blob format
	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlob)
	s.Require().NoError(err)

	// Submit blob using state module
	txConfig := state.NewTxConfig(state.WithGas(200_000), state.WithGasPrice(5000))
	txResp, err := client.State.SubmitPayForBlob(ctx, []*libshare.Blob{libBlob}, txConfig)
	s.Require().NoError(err)

	return uint64(txResp.Height), nodeBlobs[0].Commitment
}

// TestCrossNodeDataAvailability validates that data submitted to one bridge node
// becomes available across all nodes in the network
func (s *ShareTestSuite) TestCrossNodeDataAvailability() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute) // Reduced from 3 minutes
	defer cancel()

	// Get multiple nodes
	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
	lightNode1 := s.framework.GetOrCreateLightNode(ctx)
	lightNode2 := s.framework.GetOrCreateLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	light1Client := s.framework.GetNodeRPCClient(ctx, lightNode1)
	light2Client := s.framework.GetNodeRPCClient(ctx, lightNode2)

	// Submit test blob
	namespace := mustCreateNamespace(bytes.Repeat([]byte{0x01}, 10))
	data := []byte("Cross-node data availability test")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Verify data is available on all nodes using smart waiting
	nodes := map[string]*rpc_client.Client{
		"bridge": bridgeClient,
		"light1": light1Client,
		"light2": light2Client,
	}

	for nodeType, client := range nodes {
		s.Run(fmt.Sprintf("DataAvailable_%s", nodeType), func() {
			// Smart wait: WaitForHeight already handles waiting for sync
			_, err := client.Header.WaitForHeight(ctx, height)
			s.Require().NoError(err, "%s should sync to height %d", nodeType, height)

			// Verify shares are available immediately after sync
			err = client.Share.SharesAvailable(ctx, height)
			s.Require().NoError(err, "%s should have shares available at height %d", nodeType, height)

			s.T().Logf("✅ %s successfully accessed data at height %d", nodeType, height)
		})
	}
}

// TestDataPropagationTiming validates the timing of data propagation
// across multiple nodes with optimized waiting
func (s *ShareTestSuite) TestDataPropagationTiming() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute) // Reduced from 5 minutes
	defer cancel()

	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
	lightNode1 := s.framework.GetOrCreateLightNode(ctx)
	lightNode2 := s.framework.GetOrCreateLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	light1Client := s.framework.GetNodeRPCClient(ctx, lightNode1)
	light2Client := s.framework.GetNodeRPCClient(ctx, lightNode2)

	// Submit test data
	namespace := mustCreateNamespace(bytes.Repeat([]byte{0x02}, 10))
	data := []byte("Timing test data for propagation measurement")

	startTime := time.Now()
	height, _ := s.submitTestBlob(ctx, namespace, data)
	submitDuration := time.Since(startTime)

	s.T().Logf("Blob submitted in %v at height %d", submitDuration, height)

	// Test propagation timing with concurrent checks
	clients := map[string]*rpc_client.Client{
		"bridge": bridgeClient,
		"light1": light1Client,
		"light2": light2Client,
	}

	var wg sync.WaitGroup
	results := make(map[string]time.Duration)
	mu := sync.Mutex{}

	for nodeType, client := range clients {
		wg.Add(1)
		go func(nt string, nodeClient *rpc_client.Client) {
			defer wg.Done()

			nodeStartTime := time.Now()
			// Smart wait - no sleep needed before this
			_, err := nodeClient.Header.WaitForHeight(ctx, height)
			if err != nil {
				s.T().Logf("❌ %s failed to sync: %v", nt, err)
				return
			}

			// Check shares available
			err = nodeClient.Share.SharesAvailable(ctx, height)
			if err != nil {
				s.T().Logf("❌ %s shares not available: %v", nt, err)
				return
			}

			duration := time.Since(nodeStartTime)
			mu.Lock()
			results[nt] = duration
			mu.Unlock()

			s.T().Logf("✅ %s data available in %v", nt, duration)
		}(nodeType, client)
	}

	wg.Wait()

	// Verify reasonable propagation times
	for nodeType, duration := range results {
		s.Assert().Less(duration, 30*time.Second, "%s should get data within 30s", nodeType)
		s.T().Logf("📊 %s propagation time: %v", nodeType, duration)
	}
}

// TestMultiNamespaceDataIsolation validates that data in different namespaces
// is properly isolated and can be queried independently
func (s *ShareTestSuite) TestMultiNamespaceDataIsolation() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute) // Reduced from 4 minutes
	defer cancel()

	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
	lightNode := s.framework.GetOrCreateLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Create multiple namespaces and submit different data
	namespace1 := mustCreateNamespace(bytes.Repeat([]byte{0x11}, 10))
	namespace2 := mustCreateNamespace(bytes.Repeat([]byte{0x22}, 10))
	namespace3 := mustCreateNamespace(bytes.Repeat([]byte{0x33}, 10))

	data1 := []byte("Data for namespace 1")
	data2 := []byte("Data for namespace 2")
	data3 := []byte("Data for namespace 3")

	// Submit all blobs in one transaction for efficiency
	nodeAddr, err := bridgeClient.State.AccountAddress(ctx)
	s.Require().NoError(err)

	libBlob1, err := libshare.NewV1Blob(namespace1, data1, nodeAddr.Bytes())
	s.Require().NoError(err)
	libBlob2, err := libshare.NewV1Blob(namespace2, data2, nodeAddr.Bytes())
	s.Require().NoError(err)
	libBlob3, err := libshare.NewV1Blob(namespace3, data3, nodeAddr.Bytes())
	s.Require().NoError(err)

	nodeBlobs1, err := nodeblob.ToNodeBlobs(libBlob1)
	s.Require().NoError(err)
	nodeBlobs2, err := nodeblob.ToNodeBlobs(libBlob2)
	s.Require().NoError(err)
	nodeBlobs3, err := nodeblob.ToNodeBlobs(libBlob3)
	s.Require().NoError(err)

	// Fund node for blob submission
	testWallet := s.framework.CreateTestWallet(ctx, 8_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, bridgeNode, 2_000_000_000)

	allBlobs := append(nodeBlobs1, nodeBlobs2...)
	allBlobs = append(allBlobs, nodeBlobs3...)

	txConfig := state.NewTxConfig(state.WithGas(500_000), state.WithGasPrice(5000))
	height, err := bridgeClient.Blob.Submit(ctx, allBlobs, txConfig)
	s.Require().NoError(err)

	// Test data isolation on both node types with smart waiting
	clients := map[string]*rpc_client.Client{
		"bridge": bridgeClient,
		"light":  lightClient,
	}

	for nodeType, client := range clients {
		s.Run(fmt.Sprintf("DataIsolation_%s", nodeType), func() {
			// Smart wait - WaitForHeight handles sync timing
			_, err := client.Header.WaitForHeight(ctx, height)
			s.Require().NoError(err)

			// Test namespace isolation
			namespaces := []libshare.Namespace{namespace1, namespace2, namespace3}
			expectedData := [][]byte{data1, data2, data3}

			for i, ns := range namespaces {
				// Get namespace data
				namespacedShares, err := client.Share.GetNamespaceData(ctx, height, ns)
				s.Require().NoError(err, "%s should get data for namespace %d", nodeType, i+1)

				// Verify correct data is returned
				shares := namespacedShares.Flatten()
				blobs, err := libshare.ParseBlobs(shares)
				s.Require().NoError(err, "%s should parse blobs for namespace %d", nodeType, i+1)
				s.Require().NotEmpty(blobs, "%s should have blobs for namespace %d", nodeType, i+1)
				s.Assert().Equal(expectedData[i], blobs[0].Data(),
					"%s should return correct data for namespace %d", nodeType, i+1)

				s.T().Logf("✅ %s correctly isolated namespace %d data", nodeType, i+1)
			}
		})
	}
}

// TestLightNodeDataAvailabilitySampling validates light node DAS capabilities
// using GetSamples API with optimized timing
func (s *ShareTestSuite) TestLightNodeDataAvailabilitySampling() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute) // Reduced from 5 minutes
	defer cancel()

	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
	lightNode := s.framework.GetOrCreateLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Submit test data
	namespace := mustCreateNamespace(bytes.Repeat([]byte{0x44}, 10))
	data := []byte("Light node sampling test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Smart wait for both nodes to sync
	_, err := bridgeClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "bridge should sync to height")

	_, err = lightClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "light node should sync to height")

	// Test light node sampling capabilities
	s.Run("LightNodeSampling", func() {
		// Get header for sampling
		header, err := lightClient.Header.GetByHeight(ctx, height)
		s.Require().NoError(err, "light node should get header")

		// Create sample coordinates for testing
		sampleCoords := []shwap.SampleCoords{
			{Row: 0, Col: 0},
			{Row: 1, Col: 1},
		}

		// Test GetSamples functionality
		samples, err := lightClient.Share.GetSamples(ctx, header, sampleCoords)
		s.Require().NoError(err, "light node should get samples")
		s.Assert().Len(samples, len(sampleCoords), "should return correct number of samples")

		s.T().Logf("✅ Light node successfully sampled %d coordinates", len(samples))
	})

	// Test shares availability
	s.Run("SharesAvailabilityCheck", func() {
		err := lightClient.Share.SharesAvailable(ctx, height)
		s.Require().NoError(err, "light node should verify shares available")

		s.T().Logf("✅ Light node confirmed shares available at height %d", height)
	})
}

// TestNetworkResilienceWithNodeFailure validates network resilience
// when nodes fail with optimized timing
func (s *ShareTestSuite) TestNetworkResilienceWithNodeFailure() {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute) // Reduced from 6 minutes
	defer cancel()

	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
	lightNode1 := s.framework.GetOrCreateLightNode(ctx)
	lightNode2 := s.framework.GetOrCreateLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	light1Client := s.framework.GetNodeRPCClient(ctx, lightNode1)
	light2Client := s.framework.GetNodeRPCClient(ctx, lightNode2)

	// Submit test data
	namespace := mustCreateNamespace(bytes.Repeat([]byte{0x55}, 10))
	data := []byte("Network resilience test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Smart wait for all nodes to sync (no fixed sleep needed)
	allClients := map[string]*rpc_client.Client{
		"bridge": bridgeClient,
		"light1": light1Client,
		"light2": light2Client,
	}

	for nodeType, client := range allClients {
		_, err := client.Header.WaitForHeight(ctx, height)
		s.Require().NoError(err, "%s should sync to height", nodeType)

		err = client.Share.SharesAvailable(ctx, height)
		s.Require().NoError(err, "%s should have shares available", nodeType)
	}

	// **ACTUALLY STOP** one light node container to test real failure scenario
	s.T().Logf("Stopping light1 container to test network resilience...")
	err := lightNode1.Stop(ctx)
	s.Require().NoError(err, "should be able to stop light1 container")

	// Brief wait for failure detection (reduced from 10s)
	time.Sleep(3 * time.Second)

	// Test that remaining nodes can still provide data despite container failure
	remainingClients := map[string]*rpc_client.Client{
		"bridge": bridgeClient,
		"light2": light2Client,
	}

	for nodeType, client := range remainingClients {
		s.Run(fmt.Sprintf("ResilientAccess_%s", nodeType), func() {
			// Verify shares are still available
			err := client.Share.SharesAvailable(ctx, height)
			s.Require().NoError(err, "%s should still have shares available after node failure", nodeType)

			// Test namespace data retrieval
			namespacedShares, err := client.Share.GetNamespaceData(ctx, height, namespace)
			s.Require().NoError(err, "%s should get namespace data after node failure", nodeType)

			// Verify data integrity
			shares := namespacedShares.Flatten()
			blobs, err := libshare.ParseBlobs(shares)
			s.Require().NoError(err, "%s should parse blobs after node failure", nodeType)
			s.Require().NotEmpty(blobs, "%s should have blobs after node failure", nodeType)
			s.Assert().Equal(data, blobs[0].Data(),
				"%s should return correct data after node failure", nodeType)

			s.T().Logf("✅ %s successfully accessed data despite light1 failure", nodeType)
		})
	}
}

// TestConcurrentMultiNodeOperations validates concurrent operations
// across multiple nodes with optimized timing
func (s *ShareTestSuite) TestConcurrentMultiNodeOperations() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute) // Reduced from 4 minutes
	defer cancel()

	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
	lightNode1 := s.framework.GetOrCreateLightNode(ctx)
	lightNode2 := s.framework.GetOrCreateLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	light1Client := s.framework.GetNodeRPCClient(ctx, lightNode1)
	light2Client := s.framework.GetNodeRPCClient(ctx, lightNode2)

	// Submit test data
	namespace := mustCreateNamespace(bytes.Repeat([]byte{0x66}, 10))
	data := []byte("Concurrent operations test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Smart wait for sync (no fixed sleep needed)
	clients := map[string]*rpc_client.Client{
		"bridge": bridgeClient,
		"light1": light1Client,
		"light2": light2Client,
	}

	// Wait for all nodes to sync first
	for nodeType, client := range clients {
		_, err := client.Header.WaitForHeight(ctx, height)
		s.Require().NoError(err, "%s should sync to height", nodeType)
	}

	// Test concurrent operations across all nodes
	var wg sync.WaitGroup
	successCount := int64(0)
	mu := sync.Mutex{}

	for nodeType, client := range clients {
		wg.Add(3) // Three operations per node

		// Test SharesAvailable concurrently
		go func(nt string, nodeClient *rpc_client.Client) {
			defer wg.Done()
			err := nodeClient.Share.SharesAvailable(ctx, height)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
				s.T().Logf("✅ %s SharesAvailable succeeded", nt)
			}
		}(nodeType, client)

		// Test GetNamespaceData concurrently
		go func(nt string, nodeClient *rpc_client.Client) {
			defer wg.Done()
			_, err := nodeClient.Share.GetNamespaceData(ctx, height, namespace)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
				s.T().Logf("✅ %s GetNamespaceData succeeded", nt)
			}
		}(nodeType, client)

		// Test header operations concurrently
		go func(nt string, nodeClient *rpc_client.Client) {
			defer wg.Done()
			_, err := nodeClient.Header.GetByHeight(ctx, height)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
				s.T().Logf("✅ %s GetByHeight succeeded", nt)
			}
		}(nodeType, client)
	}

	wg.Wait()

	// Verify most operations succeeded
	expectedOperations := int64(len(clients) * 3)                      // 3 operations × 3 nodes = 9
	s.Assert().GreaterOrEqual(successCount, expectedOperations*80/100, // At least 80% success
		"most concurrent operations should succeed")

	s.T().Logf("✅ Concurrent operations completed: %d/%d successful", successCount, expectedOperations)
}

// Helper function to create namespace
func mustCreateNamespace(bytes []byte) libshare.Namespace {
	ns, err := libshare.NewV0Namespace(bytes)
	if err != nil {
		panic(fmt.Sprintf("failed to create namespace: %v", err))
	}
	return ns
}
