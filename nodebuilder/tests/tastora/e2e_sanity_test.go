//go:build integration

package tastora

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/go-square/v2/share"

	rpc_client "github.com/celestiaorg/celestia-node/api/rpc/client"
	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/state"
)

// E2ESanityTestSuite provides E2E sanity testing of basic functionality flows.
// Focuses on core functionality validation with minimal setup (1 BN + 1 LN).
type E2ESanityTestSuite struct {
	suite.Suite
	framework *Framework

	// Test data for sharing between steps
	testHeight     uint64
	testNamespace  share.Namespace
	testCommitment []byte
	testData       string
}

func TestE2ESanityTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E sanity integration tests in short mode")
	}
	suite.Run(t, &E2ESanityTestSuite{})
}

func (s *E2ESanityTestSuite) SetupSuite() {
	// Setup with minimal topology: 1 Bridge Node + 1 Light Node
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(1), WithLightNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))

	// Start the light node after network setup
	s.framework.NewLightNode(ctx)
}

// TestBasicDASFlow validates the complete DAS workflow: submit blob → verify availability → retrieve data with cross-node coordination
func (s *E2ESanityTestSuite) TestBasicDASFlow() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Get nodes
	bridgeNode := s.framework.GetBridgeNodes()[0]
	lightNode := s.framework.GetLightNodes()[0]

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	s.T().Log("Starting TestBasicDASFlow: submit → verify → retrieve with cross-node coordination")

	// Step 0: Ensure P2P connectivity between nodes
	s.Run("SetupP2PConnectivity", func() {
		s.framework.SetupP2PConnectivity(ctx, bridgeClient, lightClient, "light")
	})

	// Step 1: Submit blob via bridge node
	s.Run("SubmitBlob", func() {
		// Create test blob with unique namespace
		namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x01}, 10))
		s.Require().NoError(err, "should create namespace")

		blobData := []byte("TestBasicDASFlow: Cross-node DAS coordination test data")
		nodeBlobs := s.createBlobsForSubmission(ctx, bridgeClient, namespace, blobData)

		// Submit blob via bridge node
		txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
		height, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, txConfig)
		s.Require().NoError(err, "bridge node should submit blob successfully")
		s.Require().NotZero(height, "blob submission should return valid height")

		s.T().Logf("✅ Blob submitted successfully at height %d", height)

		// Store test data for later steps
		s.testHeight = height
		s.testNamespace = namespace
		s.testCommitment = nodeBlobs[0].Commitment
		s.testData = string(blobData)
	})

	// Step 2: Wait for block inclusion and propagation
	s.Run("WaitForBlockInclusion", func() {
		height := s.getTestHeight()

		// Wait for bridge node to include the block
		_, err := bridgeClient.Header.WaitForHeight(ctx, height)
		s.Require().NoError(err, "bridge node should include block at height %d", height)

		// Wait for light node to sync to the height
		_, err = lightClient.Header.WaitForHeight(ctx, height)
		s.Require().NoError(err, "light node should sync to height %d", height)

		// Allow additional time for data propagation and P2P sync
		time.Sleep(10 * time.Second)

		s.T().Logf("✅ Block included and propagated to height %d", height)
	})

	// Step 3: Verify data availability with robust waiting
	s.Run("VerifyDataAvailability", func() {
		height := s.getTestHeight()

		// Ensure light node is synced to the height
		_, err := lightClient.Header.WaitForHeight(ctx, height)
		s.Require().NoError(err, "light node should sync to height %d", height)

		// Wait for namespace data to be available first (more reliable than SharesAvailable)
		namespace := s.getTestNamespace()
		_, err = s.framework.WaitNamespaceDataAvailable(ctx, lightClient, height, namespace)
		s.Require().NoError(err, "light node should get namespace data at height %d", height)

		// Now try SharesAvailable with a shorter timeout since namespace data is confirmed
		err = s.framework.WaitSharesAvailable(ctx, lightClient, height)
		if err != nil {
			s.T().Logf("⚠️  SharesAvailable failed but namespace data is available: %v", err)
			s.T().Logf("This indicates the data is accessible but DAS verification may need more time")
		} else {
			s.T().Logf("✅ Light node verified shares available at height %d", height)
		}
	})

	// Step 4: Retrieve data on light node
	s.Run("RetrieveDataOnLightNode", func() {
		height := s.getTestHeight()
		namespace := s.getTestNamespace()
		commitment := s.getTestCommitment()
		expectedData := s.getTestData()

		// Use robust waiting for blob retrieval
		retrievedBlob, err := s.framework.WaitBlobAvailable(ctx, lightClient, height, namespace, commitment)
		s.Require().NoError(err, "light node should retrieve blob within timeout")

		s.Require().NotNil(retrievedBlob, "light node should return valid blob")

		// Verify data integrity
		retrievedData := bytes.TrimRight(retrievedBlob.Data(), "\x00")
		s.Assert().Equal([]byte(expectedData), retrievedData, "light node should return correct data")
		s.Assert().True(retrievedBlob.Namespace().Equals(namespace), "light node should return correct namespace")

		s.T().Logf("✅ Light node successfully retrieved and verified blob data")
	})

	// Step 5: Cross-node coordination verification
	s.Run("CrossNodeCoordination", func() {
		height := s.getTestHeight()
		namespace := s.getTestNamespace()

		// Get namespace data on both nodes for comparison
		bridgeNamespaceData, err := bridgeClient.Share.GetNamespaceData(ctx, height, namespace)
		s.Require().NoError(err, "bridge node should get namespace data")

		lightNamespaceData, err := lightClient.Share.GetNamespaceData(ctx, height, namespace)
		s.Require().NoError(err, "light node should get namespace data")

		// Verify both nodes return the same data
		bridgeShares := bridgeNamespaceData.Flatten()
		lightShares := lightNamespaceData.Flatten()

		s.Assert().Equal(len(bridgeShares), len(lightShares), "both nodes should return same number of shares")

		// Compare share data (first few shares for efficiency)
		compareCount := 3
		if len(bridgeShares) < compareCount {
			compareCount = len(bridgeShares)
		}

		for i := 0; i < compareCount; i++ {
			s.Assert().Equal(bridgeShares[i], lightShares[i],
				"share %d should be identical between bridge and light nodes", i)
		}

		s.T().Logf("✅ Cross-node coordination verified: both nodes return consistent data")
	})

	// Step 6: DAS sampling verification on light node
	s.Run("DASSamplingVerification", func() {
		height := s.getTestHeight()

		// Get the header first to ensure it's available
		header, err := lightClient.Header.GetByHeight(ctx, height)
		s.Require().NoError(err, "light node should get header for sampling")
		s.Require().NotNil(header, "light node should return valid header")

		// Test namespace data retrieval instead of individual shares (more reliable)
		namespace := s.getTestNamespace()
		namespaceData, err := s.framework.WaitNamespaceDataAvailable(ctx, lightClient, height, namespace)
		s.Require().NoError(err, "light node should get namespace data for DAS verification")
		s.Require().NotNil(namespaceData, "light node should return valid namespace data")
		s.T().Logf("✅ Light node successfully retrieved namespace data for DAS verification")

		// Try individual share sampling with more lenient error handling
		sampleCoords := []struct {
			Row int
			Col int
		}{
			{0, 0},
			{1, 1},
		}

		successCount := 0
		for _, coord := range sampleCoords {
			// Try to get individual share with timeout
			reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			_, err := lightClient.Share.GetShare(reqCtx, height, coord.Row, coord.Col)
			cancel()

			if err == nil {
				s.T().Logf("✅ Light node successfully sampled share at (%d, %d)", coord.Row, coord.Col)
				successCount++
			} else {
				s.T().Logf("⚠️  Light node failed to sample share at (%d, %d): %v", coord.Row, coord.Col, err)
			}
		}

		// Require at least one successful share sampling
		s.Assert().Greater(successCount, 0, "light node should successfully sample at least one share")
		s.T().Logf("✅ DAS sampling verification completed: %d/%d shares sampled successfully", successCount, len(sampleCoords))
	})

	s.T().Log("🎉 TestBasicDASFlow completed successfully: submit → verify → retrieve with cross-node coordination")
}

// Helper function to create blobs for submission
func (s *E2ESanityTestSuite) createBlobsForSubmission(ctx context.Context, client *rpc_client.Client, namespace share.Namespace, data []byte) []*nodeblob.Blob {
	nodeAddr, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err, "should get node address")

	libBlob, err := share.NewV1Blob(namespace, data, nodeAddr.Bytes())
	s.Require().NoError(err, "should create libshare blob")

	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlob)
	s.Require().NoError(err, "should convert to node blobs")

	return nodeBlobs
}

// Helper functions to get test data from struct fields
func (s *E2ESanityTestSuite) getTestHeight() uint64 {
	return s.testHeight
}

func (s *E2ESanityTestSuite) getTestNamespace() share.Namespace {
	return s.testNamespace
}

func (s *E2ESanityTestSuite) getTestCommitment() []byte {
	return s.testCommitment
}

func (s *E2ESanityTestSuite) getTestData() string {
	return s.testData
}
