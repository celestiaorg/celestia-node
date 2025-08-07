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

	"github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/state"
)

// BlobTestSuite provides integration testing of blob operations across multiple nodes.
// Focuses on cross-node data availability, network coordination, and real-world blob scenarios.
type BlobTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestBlobTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Blob module integration tests in short mode")
	}
	suite.Run(t, &BlobTestSuite{})
}

func (s *BlobTestSuite) SetupSuite() {
	// Setup with multiple nodes for comprehensive integration testing
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(1), WithLightNodes(2))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

// TestCrossNodeBlobAvailability validates that blobs submitted on one node
// become available for retrieval on other nodes in the network
func (s *BlobTestSuite) TestCrossNodeBlobAvailability() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	lightNode1 := s.framework.NewLightNode(ctx)
	lightNode2 := s.framework.NewLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	light1Client := s.framework.GetNodeRPCClient(ctx, lightNode1)
	light2Client := s.framework.GetNodeRPCClient(ctx, lightNode2)

	// Create test blob with unique namespace
	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x01}, 10))
	s.Require().NoError(err)

	blobData := []byte("Cross-node availability test data")
	nodeBlobs := s.createBlobsForSubmission(ctx, bridgeClient, namespace, blobData)

	// Submit blob via bridge node
	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	height, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err)
	s.Require().NotZero(height)

	s.T().Logf("Blob submitted at height %d", height)

	// Wait for block inclusion and propagation
	_, err = bridgeClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err)

	// Allow time for data propagation across the network
	time.Sleep(10 * time.Second)

	clients := map[string]*client.Client{
		"bridge": bridgeClient,
		"light1": light1Client,
		"light2": light2Client,
	}

	// Verify all nodes can retrieve the blob
	for nodeType, client := range clients {
		s.Run(fmt.Sprintf("BlobRetrieval_%s", nodeType), func() {
			// Ensure node is synced to the height
			_, err := client.Header.WaitForHeight(ctx, height)
			s.Require().NoError(err, "%s should sync to height %d", nodeType, height)

			// Verify blob retrieval
			retrievedBlob, err := client.Blob.Get(ctx, height, namespace, nodeBlobs[0].Commitment)
			s.Require().NoError(err, "%s should retrieve blob", nodeType)
			s.Require().NotNil(retrievedBlob, "%s should return valid blob", nodeType)

			retrievedData := bytes.TrimRight(retrievedBlob.Data(), "\x00")
			s.Assert().Equal(blobData, retrievedData, "%s should return correct data", nodeType)
			s.Assert().True(retrievedBlob.Namespace().Equals(namespace), "%s should return correct namespace", nodeType)

			s.T().Logf("✅ %s successfully retrieved blob", nodeType)
		})
	}
}

// TestMultiNamespaceBlobIsolation validates that blobs in different namespaces
// are properly isolated and can be queried independently across nodes
func (s *BlobTestSuite) TestMultiNamespaceBlobIsolation() {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	lightNode := s.framework.NewLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Create multiple namespaces with different blobs
	namespace1, err := share.NewV0Namespace(bytes.Repeat([]byte{0x11}, 10))
	s.Require().NoError(err)
	namespace2, err := share.NewV0Namespace(bytes.Repeat([]byte{0x22}, 10))
	s.Require().NoError(err)

	data1 := []byte("Namespace 1 data - Application A")
	data2 := []byte("Namespace 2 data - Application B")

	nodeBlobs1 := s.createBlobsForSubmission(ctx, bridgeClient, namespace1, data1)
	nodeBlobs2 := s.createBlobsForSubmission(ctx, bridgeClient, namespace2, data2)

	// Submit blobs to different namespaces
	txConfig := state.NewTxConfig(state.WithGas(400_000), state.WithGasPrice(5000))

	height1, err := bridgeClient.Blob.Submit(ctx, nodeBlobs1, txConfig)
	s.Require().NoError(err)

	height2, err := bridgeClient.Blob.Submit(ctx, nodeBlobs2, txConfig)
	s.Require().NoError(err)

	s.T().Logf("Blobs submitted: namespace1 at height %d, namespace2 at height %d", height1, height2)

	// Wait for both blocks
	maxHeight := height1
	if height2 > maxHeight {
		maxHeight = height2
	}

	_, err = bridgeClient.Header.WaitForHeight(ctx, maxHeight)
	s.Require().NoError(err)

	// Allow propagation time
	time.Sleep(10 * time.Second)

	clients := map[string]*client.Client{
		"bridge": bridgeClient,
		"light":  lightClient,
	}

	// Test namespace isolation on both nodes
	for nodeType, client := range clients {
		s.Run(fmt.Sprintf("NamespaceIsolation_%s", nodeType), func() {
			// Ensure node is synced
			_, err := client.Header.WaitForHeight(ctx, maxHeight)
			s.Require().NoError(err)

			// Get all blobs for namespace1 - should only return namespace1 data
			allBlobs1, err := client.Blob.GetAll(ctx, height1, []share.Namespace{namespace1})
			s.Require().NoError(err, "%s should get namespace1 blobs", nodeType)
			s.Assert().Len(allBlobs1, 1, "%s should find exactly 1 blob in namespace1", nodeType)

			if len(allBlobs1) > 0 {
				retrievedData1 := bytes.TrimRight(allBlobs1[0].Data(), "\x00")
				s.Assert().Equal(data1, retrievedData1, "%s namespace1 data should match", nodeType)
			}

			// Get all blobs for namespace2 - should only return namespace2 data
			allBlobs2, err := client.Blob.GetAll(ctx, height2, []share.Namespace{namespace2})
			s.Require().NoError(err, "%s should get namespace2 blobs", nodeType)
			s.Assert().Len(allBlobs2, 1, "%s should find exactly 1 blob in namespace2", nodeType)

			if len(allBlobs2) > 0 {
				retrievedData2 := bytes.TrimRight(allBlobs2[0].Data(), "\x00")
				s.Assert().Equal(data2, retrievedData2, "%s namespace2 data should match", nodeType)
			}

			// Cross-namespace verification - namespace1 query should not return namespace2 data
			crossBlobs, err := client.Blob.GetAll(ctx, height2, []share.Namespace{namespace1})
			s.Require().NoError(err)
			// Should be empty since namespace1 blob was submitted at height1, not height2
			s.Assert().Empty(crossBlobs, "%s should not find namespace1 blobs at height2", nodeType)

			s.T().Logf("✅ %s namespace isolation verified", nodeType)
		})
	}
}

// TestConcurrentBlobOperations validates that multiple concurrent blob operations
// work correctly across different nodes without conflicts
func (s *BlobTestSuite) TestConcurrentBlobOperations() {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	lightNode1 := s.framework.NewLightNode(ctx)
	lightNode2 := s.framework.NewLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	light1Client := s.framework.GetNodeRPCClient(ctx, lightNode1)
	light2Client := s.framework.GetNodeRPCClient(ctx, lightNode2)

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]uint64) // Map submission ID to block height

	// Concurrent blob submissions
	numSubmissions := 3
	for i := 0; i < numSubmissions; i++ {
		wg.Add(1)
		go func(submissionID int) {
			defer wg.Done()

			// Create unique namespace and data for this submission
			namespaceBytes := bytes.Repeat([]byte{byte(0x30 + submissionID)}, 10)
			namespace, err := share.NewV0Namespace(namespaceBytes)
			if err != nil {
				s.T().Errorf("Submission %d: failed to create namespace: %v", submissionID, err)
				return
			}

			data := []byte(fmt.Sprintf("Concurrent submission %d data", submissionID))
			nodeBlobs := s.createBlobsForSubmission(ctx, bridgeClient, namespace, data)

			// Submit with different gas to avoid nonce conflicts
			txConfig := state.NewTxConfig(
				state.WithGas(200_000+uint64(submissionID*10_000)),
				state.WithGasPrice(float64(5000+submissionID*100)),
			)

			height, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, txConfig)
			if err != nil {
				s.T().Errorf("Submission %d: failed to submit blob: %v", submissionID, err)
				return
			}

			mu.Lock()
			results[fmt.Sprintf("submission_%d", submissionID)] = height
			mu.Unlock()

			s.T().Logf("✅ Concurrent submission %d completed at height %d", submissionID, height)
		}(i)
	}

	// Wait for all submissions to complete
	wg.Wait()

	// Verify all submissions succeeded
	mu.Lock()
	s.Assert().Equal(numSubmissions, len(results), "All concurrent submissions should succeed")

	var maxHeight uint64
	for submissionID, height := range results {
		s.Assert().NotZero(height, "Submission %s should have valid height", submissionID)
		if height > maxHeight {
			maxHeight = height
		}
	}
	mu.Unlock()

	// Wait for all nodes to sync to the highest block
	clients := map[string]*client.Client{
		"bridge": bridgeClient,
		"light1": light1Client,
		"light2": light2Client,
	}

	for nodeType, client := range clients {
		_, err := client.Header.WaitForHeight(ctx, maxHeight)
		s.Require().NoError(err, "%s should sync to max height %d", nodeType, maxHeight)
	}

	// Allow propagation time
	time.Sleep(15 * time.Second)

	// Verify all blobs are retrievable from all nodes
	submissionCount := 0
	for submissionID, height := range results {
		submissionCount++
		submissionIdx := submissionCount - 1

		// Reconstruct the namespace for this submission
		namespaceBytes := bytes.Repeat([]byte{byte(0x30 + submissionIdx)}, 10)
		namespace, err := share.NewV0Namespace(namespaceBytes)
		s.Require().NoError(err)

		for nodeType, client := range clients {
			s.Run(fmt.Sprintf("ConcurrentRetrieval_%s_%s", nodeType, submissionID), func() {
				allBlobs, err := client.Blob.GetAll(ctx, height, []share.Namespace{namespace})
				s.Require().NoError(err, "%s should retrieve blob for %s", nodeType, submissionID)
				s.Assert().NotEmpty(allBlobs, "%s should find blobs for %s", nodeType, submissionID)

				if len(allBlobs) > 0 {
					expectedData := []byte(fmt.Sprintf("Concurrent submission %d data", submissionIdx))
					retrievedData := bytes.TrimRight(allBlobs[0].Data(), "\x00")
					s.Assert().Equal(expectedData, retrievedData,
						"%s should return correct data for %s", nodeType, submissionID)
				}
			})
		}
	}

	s.T().Logf("✅ All %d concurrent blob operations verified across all nodes", numSubmissions)
}

// TestBlobProofConsistency validates that blob inclusion proofs are consistent
// across different nodes in the network
func (s *BlobTestSuite) TestBlobProofConsistency() {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	lightNode := s.framework.NewLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Create and submit test blob
	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x05}, 10))
	s.Require().NoError(err)

	blobData := []byte("Proof consistency test data")
	nodeBlobs := s.createBlobsForSubmission(ctx, bridgeClient, namespace, blobData)

	txConfig := state.NewTxConfig(state.WithGas(250_000), state.WithGasPrice(5000))
	height, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err)

	// Wait for inclusion
	_, err = bridgeClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err)

	// Allow propagation
	time.Sleep(10 * time.Second)

	// Ensure light node is synced
	_, err = lightClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err)

	// Get proofs from both nodes
	bridgeProof, err := bridgeClient.Blob.GetProof(ctx, height, namespace, nodeBlobs[0].Commitment)
	s.Require().NoError(err, "bridge should generate proof")

	lightProof, err := lightClient.Blob.GetProof(ctx, height, namespace, nodeBlobs[0].Commitment)
	s.Require().NoError(err, "light should generate proof")

	// Verify proof consistency
	s.Assert().Equal(len(*bridgeProof), len(*lightProof), "proof lengths should match")

	for i, bridgeProofPart := range *bridgeProof {
		if i < len(*lightProof) {
			s.Assert().Equal(bridgeProofPart, (*lightProof)[i],
				"proof part %d should be identical between bridge and light", i)
		}
	}

	// Verify inclusion on both nodes
	bridgeIncluded, err := bridgeClient.Blob.Included(ctx, height, namespace, bridgeProof, nodeBlobs[0].Commitment)
	s.Require().NoError(err)
	s.Assert().True(bridgeIncluded, "bridge should verify inclusion")

	lightIncluded, err := lightClient.Blob.Included(ctx, height, namespace, lightProof, nodeBlobs[0].Commitment)
	s.Require().NoError(err)
	s.Assert().True(lightIncluded, "light should verify inclusion")

	// Cross-verification: bridge proof should work on light node
	crossIncluded, err := lightClient.Blob.Included(ctx, height, namespace, bridgeProof, nodeBlobs[0].Commitment)
	s.Require().NoError(err)
	s.Assert().True(crossIncluded, "bridge proof should verify on light node")

	s.T().Logf("✅ Blob proof consistency verified across nodes")
}

// TestLightNodeBlobAccess validates that light nodes can successfully access
// blob data without storing the full block data
func (s *BlobTestSuite) TestLightNodeBlobAccess() {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	lightNode := s.framework.NewLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Create test blob with significant data
	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x06}, 10))
	s.Require().NoError(err)

	blobData := bytes.Repeat([]byte("Light node access test "), 50) // ~1.15KB
	nodeBlobs := s.createBlobsForSubmission(ctx, bridgeClient, namespace, blobData)

	// Submit via bridge node
	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	height, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err)

	s.T().Logf("Large blob submitted at height %d, size: %d bytes", height, len(blobData))

	// Wait for inclusion and propagation
	_, err = bridgeClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err)

	time.Sleep(10 * time.Second)

	// Ensure light node is synced
	_, err = lightClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err)

	// Test light node blob access capabilities
	s.Run("LightNodeBlobRetrieval", func() {
		retrievedBlob, err := lightClient.Blob.Get(ctx, height, namespace, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "light node should retrieve blob")
		s.Require().NotNil(retrievedBlob, "light node should return valid blob")

		retrievedData := bytes.TrimRight(retrievedBlob.Data(), "\x00")
		s.Assert().Equal(blobData, retrievedData, "light node should return correct data")
		s.Assert().Equal(len(blobData), len(retrievedData), "light node should return full data")
	})

	s.Run("LightNodeBlobQuery", func() {
		allBlobs, err := lightClient.Blob.GetAll(ctx, height, []share.Namespace{namespace})
		s.Require().NoError(err, "light node should query blobs by namespace")
		s.Assert().Len(allBlobs, 1, "light node should find the blob")

		if len(allBlobs) > 0 {
			retrievedData := bytes.TrimRight(allBlobs[0].Data(), "\x00")
			s.Assert().Equal(blobData, retrievedData, "light node namespace query should return correct data")
		}
	})

	s.Run("LightNodeBlobProof", func() {
		proof, err := lightClient.Blob.GetProof(ctx, height, namespace, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "light node should generate proof")
		s.Assert().NotEmpty(proof, "light node should return valid proof")

		included, err := lightClient.Blob.Included(ctx, height, namespace, proof, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "light node should verify inclusion")
		s.Assert().True(included, "light node should confirm blob inclusion")
	})

	s.T().Logf("✅ Light node blob access capabilities verified")
}

// Helper function to create properly formatted blobs for submission
func (s *BlobTestSuite) createBlobsForSubmission(ctx context.Context, client *client.Client, namespace share.Namespace, data []byte) []*nodeblob.Blob {
	nodeAddr, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err)

	libBlob, err := share.NewV1Blob(namespace, data, nodeAddr.Bytes())
	s.Require().NoError(err)

	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlob)
	s.Require().NoError(err)

	return nodeBlobs
}
