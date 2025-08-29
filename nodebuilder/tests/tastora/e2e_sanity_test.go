//go:build integration

package tastora

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/celestia-node/header"
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

	// Step 1: Setup P2P connectivity by waiting for peers and sync wait
	s.framework.SetupP2PConnectivity(ctx, bridgeClient, lightClient, "light")

	// Create test blob with unique namespace
	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x01}, 10))
	s.Require().NoError(err, "should create namespace")

	blobData := []byte("TestBasicDASFlow: Cross-node DAS coordination test data")
	nodeBlobs := s.createBlobsForSubmission(ctx, bridgeClient, namespace, blobData)

	// Submit blob via bridge node
	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	height, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "bridge node should be able to submit blob")
	s.Require().NotZero(height, "blob submission should return valid height")

	// Store test data for later steps
	s.testHeight = height
	s.testNamespace = namespace
	s.testCommitment = nodeBlobs[0].Commitment
	s.testData = string(blobData)

	// Step 2: Wait for block inclusion and verify data availability
	// Wait for bridge node to include the block
	_, err = bridgeClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "bridge node should be able to wait for block inclusion")

	// Wait for light node to sync to the height
	_, err = lightClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "light node should be able to sync to height")

	// Verify bridge node has the shares available
	err = s.framework.WaitSharesAvailable(ctx, bridgeClient, height)
	s.Require().NoError(err, "bridge node should have shares available at height %d", height)

	// Wait for namespace data to be available on light node
	_, err = s.framework.WaitNamespaceDataAvailable(ctx, lightClient, height, namespace)
	s.Require().NoError(err, "light node should be able to get namespace data at height %d", height)

	// Step 3: Retrieve data on light node and verify cross-node coordination
	retrievedBlob, err := s.framework.WaitBlobAvailable(ctx, lightClient, height, namespace, s.testCommitment)
	s.Require().NoError(err, "light node should be able to retrieve blob within timeout")
	s.Require().NotNil(retrievedBlob, "light node should return valid blob")

	// Verify data integrity
	retrievedData := bytes.TrimRight(retrievedBlob.Data(), "\x00")
	s.Assert().Equal([]byte(s.testData), retrievedData, "light node should return correct data")

	// Step 4: Verify cross-node coordination
	bridgeNamespaceData, err := bridgeClient.Share.GetNamespaceData(ctx, height, namespace)
	s.Require().NoError(err, "bridge node should be able to get namespace data")

	lightNamespaceData, err := lightClient.Share.GetNamespaceData(ctx, height, namespace)
	s.Require().NoError(err, "light node should be able to get namespace data")

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

// TestBasicBlobLifecycle tests the complete blob lifecycle workflow on a bridge node
func (s *E2ESanityTestSuite) TestBasicBlobLifecycle() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get nodes
	bridgeNode := s.framework.GetBridgeNodes()[0]
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Step 1: Submit blob
	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x02}, 10))
	s.Require().NoError(err, "should create namespace")

	blobData := []byte("TestBasicBlobLifecycle: Complete blob lifecycle test data")
	nodeBlobs := s.createBlobsForSubmission(ctx, bridgeClient, namespace, blobData)

	// Submit blob via bridge node
	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	height, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "bridge node should be able to submit blob")
	s.Require().NotZero(height, "blob submission should return valid height")

	// Store test data for later steps
	s.testHeight = height
	s.testNamespace = namespace
	s.testCommitment = nodeBlobs[0].Commitment
	s.testData = string(blobData)

	// Step 2: Wait for inclusion and verify availability
	_, err = bridgeClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "bridge node should be able to wait for block inclusion")

	// Verify shares are available
	err = s.framework.WaitSharesAvailable(ctx, bridgeClient, height)
	s.Require().NoError(err, "bridge node should have shares available at height %d", height)

	// Step 3: Retrieve blob on same node
	retrievedBlob, err := s.framework.WaitBlobAvailable(ctx, bridgeClient, height, namespace, s.testCommitment)
	s.Require().NoError(err, "bridge node should be able to retrieve blob within timeout")
	s.Require().NotNil(retrievedBlob, "bridge node should return valid blob")

	// Verify data integrity
	retrievedData := bytes.TrimRight(retrievedBlob.Data(), "\x00")
	s.Assert().Equal([]byte(s.testData), retrievedData, "bridge node should return correct data")

	// Step 4: Get blob proof and verify inclusion
	proof, err := bridgeClient.Blob.GetProof(ctx, height, namespace, s.testCommitment)
	s.Require().NoError(err, "bridge node should be able to get blob proof")
	s.Require().NotNil(proof, "blob proof should not be nil")

	// Check if blob is included using the proof
	included, err := bridgeClient.Blob.Included(ctx, height, namespace, proof, s.testCommitment)
	s.Require().NoError(err, "bridge node should be able to verify blob inclusion")
	s.Assert().True(included, "blob should be included at height %d", height)

	// Step 5: Get all blobs at height
	blobs, err := bridgeClient.Blob.GetAll(ctx, height, []share.Namespace{namespace})
	s.Require().NoError(err, "bridge node should be able to get all blobs")
	s.Require().NotEmpty(blobs, "should return at least one blob")

	// Verify our blob is in the results
	found := false
	for _, blob := range blobs {
		if bytes.Equal(blob.Commitment, s.testCommitment) {
			found = true
			break
		}
	}
	s.Assert().True(found, "submitted blob should be found in GetAll results")
}

// TestHeaderSyncSanity tests end-to-end header synchronization workflow
func (s *E2ESanityTestSuite) TestHeaderSyncSanity() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get nodes
	bridgeNode := s.framework.GetBridgeNodes()[0]
	lightNode := s.framework.GetLightNodes()[0]

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Step 1: Verify both nodes are initially synced
	bridgeSyncState, err := bridgeClient.Header.SyncState(ctx)
	s.Require().NoError(err, "bridge node should be able to provide sync state")
	s.Require().NotNil(bridgeSyncState, "bridge sync state should not be nil")

	lightSyncState, err := lightClient.Header.SyncState(ctx)
	s.Require().NoError(err, "light node should be able to provide sync state")
	s.Require().NotNil(lightSyncState, "light sync state should not be nil")

	// Step 2: Test cross-node header consistency
	bridgeHead, err := bridgeClient.Header.LocalHead(ctx)
	s.Require().NoError(err, "bridge node should be able to provide local head")
	s.Require().NotNil(bridgeHead, "bridge head should not be nil")

	lightHead, err := lightClient.Header.LocalHead(ctx)
	s.Require().NoError(err, "light node should be able to provide local head")
	s.Require().NotNil(lightHead, "light head should not be nil")

	// Verify both nodes have the same head (they should be synced)
	s.Assert().Equal(bridgeHead.Height(), lightHead.Height(), "both nodes should have same head height")
	s.Assert().Equal(bridgeHead.Hash(), lightHead.Hash(), "both nodes should have same head hash")

	// Store current height for later use
	s.testHeight = bridgeHead.Height()

	// Step 3: Test header synchronization workflow
	currentHeight := s.testHeight
	targetHeight := currentHeight + 3

	// Wait for new blocks on bridge node
	bridgeHeader, err := bridgeClient.Header.WaitForHeight(ctx, targetHeight)
	s.Require().NoError(err, "bridge node should be able to wait for new height")
	s.Require().NotNil(bridgeHeader, "bridge header should not be nil")

	// Verify light node also syncs to the same height
	lightHeader, err := lightClient.Header.WaitForHeight(ctx, targetHeight)
	s.Require().NoError(err, "light node should be able to wait for new height")
	s.Require().NotNil(lightHeader, "light header should not be nil")

	// Verify both nodes have identical headers
	s.Assert().Equal(bridgeHeader.Hash(), lightHeader.Hash(), "both nodes should have identical headers")
	s.Assert().Equal(bridgeHeader.Height(), lightHeader.Height(), "both nodes should have same height")

	// Step 4: Test header chain consistency across nodes
	startHeight := s.testHeight - 2
	endHeight := s.testHeight

	// Get headers from both nodes and verify consistency
	bridgeHeaders := make([]*header.ExtendedHeader, 0)
	for h := startHeight; h <= endHeight; h++ {
		header, err := bridgeClient.Header.GetByHeight(ctx, h)
		s.Require().NoError(err, "bridge should be able to get header at height %d", h)
		s.Require().NotNil(header, "bridge header at height %d should not be nil", h)
		bridgeHeaders = append(bridgeHeaders, header)
	}

	lightHeaders := make([]*header.ExtendedHeader, 0)
	for h := startHeight; h <= endHeight; h++ {
		header, err := lightClient.Header.GetByHeight(ctx, h)
		s.Require().NoError(err, "light should be able to get header at height %d", h)
		s.Require().NotNil(header, "light header at height %d should not be nil", h)
		lightHeaders = append(lightHeaders, header)
	}

	// Verify all headers match between nodes
	s.Assert().Equal(len(bridgeHeaders), len(lightHeaders), "should have same number of headers")
	for i := 0; i < len(bridgeHeaders); i++ {
		s.Assert().Equal(bridgeHeaders[i].Hash(), lightHeaders[i].Hash(), "headers at index %d should match", i)
	}
}

// TestP2PConnectivity tests end-to-end P2P network connectivity workflow
func (s *E2ESanityTestSuite) TestP2PConnectivity() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get nodes
	bridgeNode := s.framework.GetBridgeNodes()[0]
	lightNode := s.framework.GetLightNodes()[0]

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Step 1: Verify nodes can establish P2P connectivity
	// This tests the core E2E workflow: nodes should be able to connect to each other
	bridgeInfo, err := bridgeClient.P2P.Info(ctx)
	s.Require().NoError(err, "bridge node should be able to provide P2P info")
	s.Require().NotNil(bridgeInfo, "bridge P2P info should not be nil")

	lightInfo, err := lightClient.P2P.Info(ctx)
	s.Require().NoError(err, "light node should be able to provide P2P info")
	s.Require().NotNil(lightInfo, "light P2P info should not be nil")

	// Step 2: Test the core E2E workflow - nodes should discover each other
	bridgePeers, err := bridgeClient.P2P.Peers(ctx)
	s.Require().NoError(err, "bridge node should be able to discover peers")
	s.Require().NotNil(bridgePeers, "bridge peer list should not be nil")

	lightPeers, err := lightClient.P2P.Peers(ctx)
	s.Require().NoError(err, "light node should be able to discover peers")
	s.Require().NotNil(lightPeers, "light peer list should not be nil")

	// Step 3: Test the core E2E workflow - nodes should be connected to each other
	// This is the main E2E test: verify that the network topology works
	s.Assert().GreaterOrEqual(len(bridgePeers), 1, "bridge node should have discovered at least one peer")
	s.Assert().GreaterOrEqual(len(lightPeers), 1, "light node should have discovered at least one peer")

	// Step 4: Test the core E2E workflow - bidirectional connectivity
	// This verifies that the P2P network is functioning end-to-end
	bridgeConnectivity, err := bridgeClient.P2P.Connectedness(ctx, lightInfo.ID)
	s.Require().NoError(err, "bridge node should be able to check connectivity")
	s.Assert().Equal(1, int(bridgeConnectivity), "bridge node should be connected to light node")

	lightConnectivity, err := lightClient.P2P.Connectedness(ctx, bridgeInfo.ID)
	s.Require().NoError(err, "light node should be able to check connectivity")
	s.Assert().Equal(1, int(lightConnectivity), "light node should be connected to bridge node")

	// Step 5: Test the core E2E workflow - network is functional for data exchange
	// This verifies that the P2P network can support actual data exchange
	bridgePeerInfo, err := bridgeClient.P2P.PeerInfo(ctx, lightInfo.ID)
	s.Require().NoError(err, "bridge node should be able to exchange peer information")
	s.Require().NotNil(bridgePeerInfo, "bridge should receive valid peer info")

	lightPeerInfo, err := lightClient.P2P.PeerInfo(ctx, bridgeInfo.ID)
	s.Require().NoError(err, "light node should be able to exchange peer information")
	s.Require().NotNil(lightPeerInfo, "light should receive valid peer info")
}
