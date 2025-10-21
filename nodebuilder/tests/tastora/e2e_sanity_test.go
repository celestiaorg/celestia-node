//go:build integration

package tastora

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/go-square/v3/share"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/state"
)

// E2ESanityTestSuite provides E2E sanity testing of basic functionality flows.
// Focuses on core functionality validation with minimal setup (1 BN + 1 LN).
type E2ESanityTestSuite struct {
	suite.Suite
	framework *Framework
	timeout   time.Duration
}

func TestE2ESanityTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E sanity integration tests in short mode")
	}
	suite.Run(t, &E2ESanityTestSuite{
		timeout: 2 * time.Minute, // Default timeout
	})
}

// NewE2ESanityTestSuite creates a new test suite with custom timeout
func NewE2ESanityTestSuite(timeout time.Duration) *E2ESanityTestSuite {
	return &E2ESanityTestSuite{
		timeout: timeout,
	}
}

// withTimeout creates a context with the suite's configured timeout
func (s *E2ESanityTestSuite) withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), s.timeout)
}

func (s *E2ESanityTestSuite) SetupSuite() {
	// Setup with minimal topology: 1 Bridge Node + 1 Light Node
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(1), WithLightNodes(1))
	ctx, cancel := s.withTimeout()
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))

	// Start the light node after network setup
	s.framework.NewLightNode(ctx)
}

func (s *E2ESanityTestSuite) TearDownSuite() {
	if s.framework != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.framework.Stop(ctx)
	}
}

// TestBasicDASFlow validates basic Data Availability Sampling functionality on a single bridge node
func (s *E2ESanityTestSuite) TestBasicDASFlow() {
	ctx, cancel := s.withTimeout()
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x01}, 10))
	s.Require().NoError(err, "should create namespace")

	blobData := []byte("TestBasicDASFlow: Basic DAS functionality test data")
	nodeBlobs := s.createBlobsForSubmission(ctx, bridgeClient, namespace, blobData)

	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	height, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "bridge node should be able to submit blob")
	s.Require().NotZero(height, "blob submission should return valid height")

	_, err = bridgeClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "bridge node should be able to wait for block inclusion")

	err = bridgeClient.Share.SharesAvailable(ctx, height)
	s.Require().NoError(err, "bridge node should have shares available at height %d", height)
}

// Helper function to create blobs for submission
func (s *E2ESanityTestSuite) createBlobsForSubmission(ctx context.Context, client *rpcclient.Client, namespace share.Namespace, data []byte) []*nodeblob.Blob {
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
	ctx, cancel := s.withTimeout()
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x02}, 10))
	s.Require().NoError(err, "should create namespace")

	blobData := []byte("TestBasicBlobLifecycle: Complete blob lifecycle test data")
	nodeBlobs := s.createBlobsForSubmission(ctx, bridgeClient, namespace, blobData)

	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	height, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "bridge node should be able to submit blob")
	s.Require().NotZero(height, "blob submission should return valid height")

	_, err = bridgeClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "bridge node should be able to wait for block inclusion")

	retrievedBlob, err := bridgeClient.Blob.Get(ctx, height, namespace, nodeBlobs[0].Commitment)
	s.Require().NoError(err, "bridge node should be able to retrieve blob within timeout")
	s.Require().NotNil(retrievedBlob, "bridge node should return valid blob")

	retrievedData := bytes.TrimRight(retrievedBlob.Data(), "\x00")
	s.Assert().Equal(blobData, retrievedData, "bridge node should return correct data")

	proof, err := bridgeClient.Blob.GetProof(ctx, height, namespace, nodeBlobs[0].Commitment)
	s.Require().NoError(err, "bridge node should be able to get blob proof")
	s.Require().NotNil(proof, "blob proof should not be nil")

	included, err := bridgeClient.Blob.Included(ctx, height, namespace, proof, nodeBlobs[0].Commitment)
	s.Require().NoError(err, "bridge node should be able to verify blob inclusion")
	s.Assert().True(included, "blob should be included at height %d", height)

	blobs, err := bridgeClient.Blob.GetAll(ctx, height, []share.Namespace{namespace})
	s.Require().NoError(err, "bridge node should be able to get all blobs")
	s.Require().NotEmpty(blobs, "should return at least one blob")

	found := false
	for _, blob := range blobs {
		if bytes.Equal(blob.Commitment, nodeBlobs[0].Commitment) {
			found = true
			break
		}
	}
	s.Assert().True(found, "submitted blob should be found in GetAll results")
}

// TestHeaderSyncSanity tests end-to-end header synchronization workflow
func (s *E2ESanityTestSuite) TestHeaderSyncSanity() {
	ctx, cancel := s.withTimeout()
	defer cancel()

	// Get nodes
	bridgeNode := s.framework.GetBridgeNodes()[0]
	lightNode := s.framework.GetLightNodes()[0]

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	bridgeSyncState, err := bridgeClient.Header.SyncState(ctx)
	s.Require().NoError(err, "bridge node should be able to provide sync state")
	s.Require().NotNil(bridgeSyncState, "bridge sync state should not be nil")

	lightSyncState, err := lightClient.Header.SyncState(ctx)
	s.Require().NoError(err, "light node should be able to provide sync state")
	s.Require().NotNil(lightSyncState, "light sync state should not be nil")

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
	currentHeight := bridgeHead.Height()
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

	startHeight := currentHeight - 2
	endHeight := currentHeight

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

	s.Assert().Equal(len(bridgeHeaders), len(lightHeaders), "should have same number of headers")
	for i := 0; i < len(bridgeHeaders); i++ {
		s.Assert().Equal(bridgeHeaders[i].Hash(), lightHeaders[i].Hash(), "headers at index %d should match", i)
	}
}

// TestP2PConnectivity tests end-to-end P2P network connectivity workflow
func (s *E2ESanityTestSuite) TestP2PConnectivity() {
	ctx, cancel := s.withTimeout()
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	lightNode := s.framework.GetLightNodes()[0]

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	bridgeInfo, err := bridgeClient.P2P.Info(ctx)
	s.Require().NoError(err, "bridge node should be able to provide P2P info")
	s.Require().NotNil(bridgeInfo, "bridge P2P info should not be nil")

	lightInfo, err := lightClient.P2P.Info(ctx)
	s.Require().NoError(err, "light node should be able to provide P2P info")
	s.Require().NotNil(lightInfo, "light P2P info should not be nil")

	// Verify initial connectivity
	bridgeConnectivity, err := bridgeClient.P2P.Connectedness(ctx, lightInfo.ID)
	s.Require().NoError(err, "bridge node should be able to check connectivity")
	s.Assert().Equal(1, int(bridgeConnectivity), "bridge node should be connected to light node initially")

	lightConnectivity, err := lightClient.P2P.Connectedness(ctx, bridgeInfo.ID)
	s.Require().NoError(err, "light node should be able to check connectivity")
	s.Assert().Equal(1, int(lightConnectivity), "light node should be connected to bridge node initially")

	// Test disconnect operation - close connection from bridge to light node
	s.T().Log("Testing P2P disconnect operation")
	err = bridgeClient.P2P.ClosePeer(ctx, lightInfo.ID)
	s.Require().NoError(err, "bridge node should be able to close connection to light node")

	// Wait a moment for the disconnect to propagate
	time.Sleep(3 * time.Second)

	// Verify disconnect was successful (or at least attempted)
	bridgeConnectivity, err = bridgeClient.P2P.Connectedness(ctx, lightInfo.ID)
	s.Require().NoError(err, "bridge node should be able to check connectivity after disconnect")

	// Note: In some P2P configurations, connections may auto-reconnect
	// We'll test that the ClosePeer operation succeeds and then test Connect
	s.T().Logf("Bridge connectivity after ClosePeer: %d", int(bridgeConnectivity))

	// Test connect operation - ensure we can explicitly connect
	s.T().Log("Testing P2P connect operation")
	err = bridgeClient.P2P.Connect(ctx, lightInfo)
	s.Require().NoError(err, "bridge node should be able to connect to light node")

	// Wait a moment for the connection to establish
	time.Sleep(3 * time.Second)

	// Verify connect operation was successful
	bridgeConnectivity, err = bridgeClient.P2P.Connectedness(ctx, lightInfo.ID)
	s.Require().NoError(err, "bridge node should be able to check connectivity after connect")
	s.Assert().Equal(1, int(bridgeConnectivity), "bridge node should be connected to light node after explicit connect")

	lightConnectivity, err = lightClient.P2P.Connectedness(ctx, bridgeInfo.ID)
	s.Require().NoError(err, "light node should be able to check connectivity after connect")
	s.Assert().Equal(1, int(lightConnectivity), "light node should be connected to bridge node after explicit connect")

	// Verify peer information exchange still works after connect
	bridgePeerInfo, err := bridgeClient.P2P.PeerInfo(ctx, lightInfo.ID)
	s.Require().NoError(err, "bridge node should be able to exchange peer information after connect")
	s.Require().NotNil(bridgePeerInfo, "bridge should receive valid peer info after connect")

	lightPeerInfo, err := lightClient.P2P.PeerInfo(ctx, bridgeInfo.ID)
	s.Require().NoError(err, "light node should be able to exchange peer information after connect")
	s.Require().NotNil(lightPeerInfo, "light should receive valid peer info after connect")

	// Verify peers are listed correctly
	bridgePeers, err := bridgeClient.P2P.Peers(ctx)
	s.Require().NoError(err, "bridge node should be able to discover peers after connect")
	s.Require().NotNil(bridgePeers, "bridge peer list should not be nil after connect")

	lightPeers, err := lightClient.P2P.Peers(ctx)
	s.Require().NoError(err, "light node should be able to discover peers after connect")
	s.Require().NotNil(lightPeers, "light peer list should not be nil after connect")

	s.Assert().GreaterOrEqual(len(bridgePeers), 1, "bridge node should have discovered at least one peer after connect")
	s.Assert().GreaterOrEqual(len(lightPeers), 1, "light node should have discovered at least one peer after connect")
}
