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

	"github.com/celestiaorg/celestia-node/api/rpc/client"
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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

	// Create and submit blob
	nodeAddr, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err)

	libBlob, err := libshare.NewV1Blob(namespace, data, nodeAddr.Bytes())
	s.Require().NoError(err)

	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlob)
	s.Require().NoError(err)

	commitmentBytes := nodeBlobs[0].Commitment

	txConfig := state.NewTxConfig(state.WithGas(200_000), state.WithGasPrice(5000))
	height, err := client.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err)

	return height, commitmentBytes
}

// mustCreateNamespace creates a namespace and panics on error (for test data setup)
func mustCreateNamespace(id []byte) libshare.Namespace {
	ns, err := libshare.NewV0Namespace(id)
	if err != nil {
		panic(fmt.Sprintf("failed to create namespace: %v", err))
	}
	return ns
}

// TestCrossNodeDataAvailability validates that data submitted to one bridge node
// becomes available across all nodes in the network
func (s *ShareTestSuite) TestCrossNodeDataAvailability() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Get multiple bridge nodes and light nodes
	bridgeNode1 := s.framework.GetOrCreateBridgeNode(ctx)
	bridgeNode2 := s.framework.GetOrCreateBridgeNode(ctx)
	lightNode1 := s.framework.GetOrCreateLightNode(ctx)
	lightNode2 := s.framework.GetOrCreateLightNode(ctx)

	bridge1Client := s.framework.GetNodeRPCClient(ctx, bridgeNode1)
	bridge2Client := s.framework.GetNodeRPCClient(ctx, bridgeNode2)
	light1Client := s.framework.GetNodeRPCClient(ctx, lightNode1)
	light2Client := s.framework.GetNodeRPCClient(ctx, lightNode2)

	// Submit test blob to bridge1
	namespace := mustCreateNamespace(bytes.Repeat([]byte{0x01}, 10))
	data := []byte("Cross-node data availability test")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Wait for block propagation
	time.Sleep(15 * time.Second)

	// Verify data is available on all nodes
	nodes := map[string]*client.Client{
		"bridge1": bridge1Client,
		"bridge2": bridge2Client,
		"light1":  light1Client,
		"light2":  light2Client,
	}

	for nodeType, client := range nodes {
		s.Run(fmt.Sprintf("DataAvailable_%s", nodeType), func() {
			// Wait for node to sync to height
			_, err := client.Header.WaitForHeight(ctx, height)
			s.Require().NoError(err, "%s should sync to height %d", nodeType, height)

			// Check shares are available
			err = client.Share.SharesAvailable(ctx, height)
			s.Require().NoError(err, "shares should be available on %s", nodeType)

			// Get namespace data to verify content
			namespaceData, err := client.Share.GetNamespaceData(ctx, height, namespace)
			s.Require().NoError(err, "%s should get namespace data", nodeType)

			allShares := namespaceData.Flatten()
			s.Assert().NotEmpty(allShares, "%s should have shares for namespace", nodeType)

			// Verify data integrity
			blobs, err := libshare.ParseBlobs(allShares)
			s.Require().NoError(err, "%s should parse blobs", nodeType)
			s.Assert().NotEmpty(blobs, "%s should have parsed blobs", nodeType)
		})
	}
}

// TestDataPropagationTiming validates the timing of data propagation across different node types
func (s *ShareTestSuite) TestDataPropagationTiming() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
	lightNode := s.framework.GetOrCreateLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Submit test blob
	namespace := mustCreateNamespace(bytes.Repeat([]byte{0x02}, 10))
	data := []byte("Data propagation timing test")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Track when data becomes available on each node type
	type availabilityResult struct {
		nodeType  string
		timestamp time.Time
		duration  time.Duration
	}

	var results []availabilityResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	startTime := time.Now()

	// Check bridge node availability
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				err := bridgeClient.Share.SharesAvailable(ctx, height)
				if err == nil {
					mu.Lock()
					results = append(results, availabilityResult{
						nodeType:  "bridge",
						timestamp: time.Now(),
						duration:  time.Since(startTime),
					})
					mu.Unlock()
					return
				}
				time.Sleep(1 * time.Second)
			}
		}
	}()

	// Check light node availability
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Wait for header sync first
		_, err := lightClient.Header.WaitForHeight(ctx, height)
		if err != nil {
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			default:
				err := lightClient.Share.SharesAvailable(ctx, height)
				if err == nil {
					mu.Lock()
					results = append(results, availabilityResult{
						nodeType:  "light",
						timestamp: time.Now(),
						duration:  time.Since(startTime),
					})
					mu.Unlock()
					return
				}
				time.Sleep(1 * time.Second)
			}
		}
	}()

	wg.Wait()

	// Validate results
	s.Require().Len(results, 2, "should have availability results for both node types")

	var bridgeDuration, lightDuration time.Duration
	for _, result := range results {
		switch result.nodeType {
		case "bridge":
			bridgeDuration = result.duration
		case "light":
			lightDuration = result.duration
		}
	}

	s.T().Logf("Bridge node data availability: %v", bridgeDuration)
	s.T().Logf("Light node data availability: %v", lightDuration)

	// Bridge should be faster (it stores full data)
	s.Assert().True(bridgeDuration <= lightDuration, "bridge node should have data available before or same time as light node")
}

// TestMultiNamespaceDataIsolation validates that different namespaces are properly isolated
// and each node can correctly filter and retrieve namespace-specific data
func (s *ShareTestSuite) TestMultiNamespaceDataIsolation() {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
	lightNode := s.framework.GetOrCreateLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Create test wallets and fund accounts
	testWallet := s.framework.CreateTestWallet(ctx, 10_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, bridgeNode, 2_000_000_000)

	// Submit blobs to different namespaces
	namespace1 := mustCreateNamespace(bytes.Repeat([]byte{0x11}, 10))
	namespace2 := mustCreateNamespace(bytes.Repeat([]byte{0x22}, 10))
	namespace3 := mustCreateNamespace(bytes.Repeat([]byte{0x33}, 10))

	data1 := []byte("Namespace 1 data")
	data2 := []byte("Namespace 2 data")
	data3 := []byte("Namespace 3 data")

	// Submit all blobs in a single transaction for same block height
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

	allBlobs := append(nodeBlobs1, nodeBlobs2...)
	allBlobs = append(allBlobs, nodeBlobs3...)

	txConfig := state.NewTxConfig(state.WithGas(500_000), state.WithGasPrice(5000))
	height, err := bridgeClient.Blob.Submit(ctx, allBlobs, txConfig)
	s.Require().NoError(err)

	// Wait for data propagation
	time.Sleep(20 * time.Second)

	// Test data isolation on both node types
	clients := map[string]*client.Client{
		"bridge": bridgeClient,
		"light":  lightClient,
	}

	for nodeType, client := range clients {
		s.Run(fmt.Sprintf("DataIsolation_%s", nodeType), func() {
			// Wait for sync
			_, err := client.Header.WaitForHeight(ctx, height)
			s.Require().NoError(err)

			// Get namespace data for each namespace
			ns1Data, err := client.Share.GetNamespaceData(ctx, height, namespace1)
			s.Require().NoError(err, "%s should get namespace1 data", nodeType)

			ns2Data, err := client.Share.GetNamespaceData(ctx, height, namespace2)
			s.Require().NoError(err, "%s should get namespace2 data", nodeType)

			ns3Data, err := client.Share.GetNamespaceData(ctx, height, namespace3)
			s.Require().NoError(err, "%s should get namespace3 data", nodeType)

			// Verify each namespace has its own data
			ns1Shares := ns1Data.Flatten()
			ns2Shares := ns2Data.Flatten()
			ns3Shares := ns3Data.Flatten()

			s.Assert().NotEmpty(ns1Shares, "%s should have namespace1 shares", nodeType)
			s.Assert().NotEmpty(ns2Shares, "%s should have namespace2 shares", nodeType)
			s.Assert().NotEmpty(ns3Shares, "%s should have namespace3 shares", nodeType)

			// Verify namespace isolation - shares should belong to correct namespace
			for _, share := range ns1Shares {
				s.Assert().True(share.Namespace().Equals(namespace1),
					"%s namespace1 share should belong to namespace1", nodeType)
			}
			for _, share := range ns2Shares {
				s.Assert().True(share.Namespace().Equals(namespace2),
					"%s namespace2 share should belong to namespace2", nodeType)
			}
			for _, share := range ns3Shares {
				s.Assert().True(share.Namespace().Equals(namespace3),
					"%s namespace3 share should belong to namespace3", nodeType)
			}

			// Parse and verify blob content
			blobs1, err := libshare.ParseBlobs(ns1Shares)
			s.Require().NoError(err)
			blobs2, err := libshare.ParseBlobs(ns2Shares)
			s.Require().NoError(err)
			blobs3, err := libshare.ParseBlobs(ns3Shares)
			s.Require().NoError(err)

			s.Assert().NotEmpty(blobs1, "%s should have namespace1 blobs", nodeType)
			s.Assert().NotEmpty(blobs2, "%s should have namespace2 blobs", nodeType)
			s.Assert().NotEmpty(blobs3, "%s should have namespace3 blobs", nodeType)
		})
	}
}

// TestLightNodeDataAvailabilitySampling validates that light nodes can successfully
// perform Data Availability Sampling (DAS) across multiple bridge nodes
func (s *ShareTestSuite) TestLightNodeDataAvailabilitySampling() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Use light node to test DAS
	lightNode := s.framework.GetOrCreateLightNode(ctx)

	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Submit larger blob to ensure meaningful sampling
	namespace := mustCreateNamespace(bytes.Repeat([]byte{0x44}, 10))
	// Create larger data to ensure multiple shares
	largeData := bytes.Repeat([]byte("DAS test data "), 1000) // ~14KB
	height, _ := s.submitTestBlob(ctx, namespace, largeData)

	// Wait for data propagation
	time.Sleep(25 * time.Second)

	// Verify light node can perform DAS
	_, err := lightClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "light node should sync to height")

	// Check that light node considers shares available (DAS successful)
	err = lightClient.Share.SharesAvailable(ctx, height)
	s.Require().NoError(err, "light node should confirm shares available via DAS")

	// Test sampling by requesting different share ranges
	// Light node should be able to get samples even though it doesn't store full data
	header, err := lightClient.Header.GetByHeight(ctx, height)
	s.Require().NoError(err, "should get header")

	for i := 0; i < 3; i++ {
		samples, err := lightClient.Share.GetSamples(ctx, header, []shwap.SampleCoords{
			{Row: i, Col: i},
		})
		s.Require().NoError(err, "light node should get samples via DAS")
		s.Assert().NotEmpty(samples, "light node should receive sample data")
		s.Assert().Len(samples, 1, "should receive exactly one sample")
	}

	// Verify light node can get namespace data (may use ShrexND protocol)
	namespaceData, err := lightClient.Share.GetNamespaceData(ctx, height, namespace)
	s.Require().NoError(err, "light node should get namespace data")

	shares := namespaceData.Flatten()
	s.Assert().NotEmpty(shares, "light node should have namespace shares")

	// Verify data integrity
	blobs, err := libshare.ParseBlobs(shares)
	s.Require().NoError(err, "light node should parse blobs")
	s.Assert().NotEmpty(blobs, "light node should have parsed blobs")
}

// TestNetworkResilienceWithNodeFailure tests how the network handles node failures
// and validates data availability when some nodes are unavailable
func (s *ShareTestSuite) TestNetworkResilienceWithNodeFailure() {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	// Get bridge node and multiple light nodes for resilience testing
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

	// Wait for data propagation to all nodes
	time.Sleep(30 * time.Second)

	// Verify all nodes have the data initially
	allClients := map[string]*client.Client{
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

	// Wait for failure detection and network adaptation
	time.Sleep(10 * time.Second)

	// Test that remaining nodes can still provide data despite container failure
	remainingClients := map[string]*client.Client{
		"bridge": bridgeClient,
		"light2": light2Client,
	}

	for nodeType, client := range remainingClients {
		s.Run(fmt.Sprintf("ResilientAccess_%s", nodeType), func() {
			// Verify shares are still available
			err := client.Share.SharesAvailable(ctx, height)
			s.Require().NoError(err, "%s should still have shares available", nodeType)

			// Verify we can still get the namespace data
			namespaceData, err := client.Share.GetNamespaceData(ctx, height, namespace)
			s.Require().NoError(err, "%s should still get namespace data", nodeType)

			shares := namespaceData.Flatten()
			s.Assert().NotEmpty(shares, "%s should still have shares", nodeType)

			// Verify data integrity
			blobs, err := libshare.ParseBlobs(shares)
			s.Require().NoError(err, "%s should still parse blobs", nodeType)
			s.Assert().NotEmpty(blobs, "%s should still have blobs", nodeType)
		})
	}

	s.T().Logf("✅ Network resilience test completed: real container failure tested!")
}

// TestConcurrentMultiNodeOperations validates that multiple nodes can handle
// concurrent operations without conflicts or data corruption
func (s *ShareTestSuite) TestConcurrentMultiNodeOperations() {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	bridgeNode1 := s.framework.GetOrCreateBridgeNode(ctx)
	bridgeNode2 := s.framework.GetOrCreateBridgeNode(ctx)
	lightNode1 := s.framework.GetOrCreateLightNode(ctx)
	lightNode2 := s.framework.GetOrCreateLightNode(ctx)

	bridge1Client := s.framework.GetNodeRPCClient(ctx, bridgeNode1)
	bridge2Client := s.framework.GetNodeRPCClient(ctx, bridgeNode2)
	light1Client := s.framework.GetNodeRPCClient(ctx, lightNode1)
	light2Client := s.framework.GetNodeRPCClient(ctx, lightNode2)

	// Submit test data
	namespace := mustCreateNamespace(bytes.Repeat([]byte{0x66}, 10))
	data := []byte("Concurrent operations test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Wait for data propagation
	time.Sleep(20 * time.Second)

	// Perform concurrent operations across all nodes
	var wg sync.WaitGroup
	errors := make(chan error, 12) // 4 nodes × 3 operations each

	clients := []*client.Client{bridge1Client, bridge2Client, light1Client, light2Client}
	nodeNames := []string{"bridge1", "bridge2", "light1", "light2"}

	for i, nodeClient := range clients {
		nodeName := nodeNames[i]

		// Concurrent SharesAvailable calls
		wg.Add(1)
		go func(c *client.Client, name string) {
			defer wg.Done()
			err := c.Share.SharesAvailable(ctx, height)
			if err != nil {
				errors <- fmt.Errorf("%s SharesAvailable: %w", name, err)
			}
		}(nodeClient, nodeName)

		// Concurrent GetNamespaceData calls
		wg.Add(1)
		go func(c *client.Client, name string) {
			defer wg.Done()
			_, err := c.Share.GetNamespaceData(ctx, height, namespace)
			if err != nil {
				errors <- fmt.Errorf("%s GetNamespaceData: %w", name, err)
			}
		}(nodeClient, nodeName)

		// Concurrent GetRange calls
		wg.Add(1)
		go func(c *client.Client, name string) {
			defer wg.Done()
			_, err := c.Share.GetRange(ctx, height, 0, 2)
			if err != nil {
				errors <- fmt.Errorf("%s GetRange: %w", name, err)
			}
		}(nodeClient, nodeName)
	}

	// Wait for all operations to complete
	wg.Wait()
	close(errors)

	// Check for errors
	var errorList []error
	for err := range errors {
		errorList = append(errorList, err)
	}

	s.Assert().Empty(errorList, "concurrent operations should succeed on all nodes: %v", errorList)
}
