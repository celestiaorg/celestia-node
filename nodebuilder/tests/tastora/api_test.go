//go:build integration

package tastora

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/go-square/v2/share"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	nodeblob "github.com/celestiaorg/celestia-node/blob"
	nodeshare "github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/state"
	"github.com/filecoin-project/go-jsonrpc/auth"
)

// APITestSuite provides pure API contract validation for rapid feedback.
// Focuses on API compliance, RPC contract validation, and parameter validation.
type APITestSuite struct {
	suite.Suite
	framework *Framework
}

func TestAPITestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping API integration tests in short mode")
	}
	suite.Run(t, &APITestSuite{})
}

func (s *APITestSuite) SetupSuite() {
	// Setup with minimal topology: 1 Bridge Node + 1 Light Node (allocated but not started)
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(1), WithLightNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

// TestRPCEndpointCompliance validates raw RPC endpoint compliance for both BN/LN
func (s *APITestSuite) TestRPCEndpointCompliance() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Create a light node for RPC compliance testing
	lightNode := s.framework.NewLightNode(ctx)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	s.Run("BridgeNodeRPCCompliance", func() {
		info, err := bridgeClient.Node.Info(ctx)
		s.Require().NoError(err, "bridge node should provide node info")
		s.Require().NotNil(info, "bridge node info should not be nil")

		ready, err := bridgeClient.Node.Ready(ctx)
		s.Require().NoError(err, "bridge node should provide readiness status")
		s.Require().True(ready, "bridge node should be ready")
	})

	s.Run("LightNodeRPCCompliance", func() {
		info, err := lightClient.Node.Info(ctx)
		s.Require().NoError(err, "light node should provide node info")
		s.Require().NotNil(info, "light node info should not be nil")

		ready, err := lightClient.Node.Ready(ctx)
		s.Require().NoError(err, "light node should provide readiness status")
		s.Require().True(ready, "light node should be ready")
	})
}

// TestShareAPIContract validates Share module API responses match expected schema
func (s *APITestSuite) TestShareAPIContract() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Create light node for testing
	lightNode := s.framework.NewLightNode(ctx)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Wait for DHT to stabilize and light node to be ready
	time.Sleep(20 * time.Second)

	// Submit blob data to ensure there's actual share data to test with
	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x01}, 10))
	s.Require().NoError(err, "should create namespace")

	blobData := []byte("TestShareAPIContract: Share API validation test data")
	nodeBlobs := s.createBlobsForSubmission(ctx, bridgeClient, namespace, blobData)

	// Submit blob for testing
	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	height, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "should be able to submit blob for Share API testing")
	s.Require().NotZero(height, "blob submission should return valid height")

	// Wait for inclusion and light node sync
	_, err = bridgeClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "should wait for blob inclusion")

	_, err = lightClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "light node should be synced to sufficient height")

	// Get the actual header and DAH to find valid coordinates for testing
	header, err := bridgeClient.Header.GetByHeight(ctx, height)
	s.Require().NoError(err, "should be able to get header at target height")
	s.Require().NotNil(header, "header should not be nil")
	s.Require().NotNil(header.DAH, "DAH should not be nil")

	// Verify DAH structure
	dahSize := len(header.DAH.RowRoots)
	s.Require().Greater(dahSize, 0, "DAH should have at least one row")

	s.Run("GetNamespaceData", func() {
		// Test bridge node first (always works)
		bridgeData, err := bridgeClient.Share.GetNamespaceData(ctx, height, namespace)
		s.Require().NoError(err, "bridge node GetNamespaceData should succeed for valid namespace")
		s.Require().NotNil(bridgeData, "bridge node GetNamespaceData should return valid data")

		// Test light node GetNamespaceData
		lightData, err := lightClient.Share.GetNamespaceData(ctx, height, namespace)
		s.Require().NoError(err, "light node GetNamespaceData should succeed for valid namespace")
		s.Require().NotNil(lightData, "light node GetNamespaceData should return valid data")
	})

	s.Run("SharesAvailable", func() {
		// Test bridge node SharesAvailable (always works)
		err := bridgeClient.Share.SharesAvailable(ctx, height)
		s.Require().NoError(err, "bridge node SharesAvailable should succeed for valid height")

		// Test light node SharesAvailable with polling (bitswap-dependent)
		lightSharesAvailable := s.pollSharesAvailable(ctx, lightClient, height, 45*time.Second)
		s.Require().True(lightSharesAvailable, "light node SharesAvailable should succeed")
	})

	s.Run("GetSamples", func() {
		// Use valid coordinates within the DAH size (0-based indexing)
		coords := []shwap.SampleCoords{{Row: 0, Col: 0}}

		// Test bridge node first (always works)
		bridgeSamples, err := bridgeClient.Share.GetSamples(ctx, height, coords)
		s.Require().NoError(err, "bridge node GetSamples should succeed for valid coordinates")
		s.Require().Len(bridgeSamples, 1, "bridge node GetSamples should return expected number of samples")

		// Test light node GetSamples with polling (bitswap-dependent)
		lightSamples := s.pollGetSamples(ctx, lightClient, height, coords, 45*time.Second)
		s.Require().NotNil(lightSamples, "light node GetSamples should succeed for valid coordinates")
		s.Require().Len(lightSamples, 1, "light node GetSamples should return expected number of samples")
	})

	s.Run("GetEDS", func() {
		// Test bridge node first (always works)
		bridgeEds, err := bridgeClient.Share.GetEDS(ctx, height)
		s.Require().NoError(err, "bridge node GetEDS should succeed for valid height")
		s.Require().NotNil(bridgeEds, "bridge node GetEDS should return valid EDS")

		// Test light node GetEDS
		lightEds, err := lightClient.Share.GetEDS(ctx, height)
		s.Require().NoError(err, "light node GetEDS should succeed for valid height")
		s.Require().NotNil(lightEds, "light node GetEDS should return valid EDS")
	})

	s.Run("GetRow", func() {
		// Use valid row index (0-based indexing)
		testRow := 0 // First row

		// Test bridge node first (always works)
		bridgeRow, err := bridgeClient.Share.GetRow(ctx, height, testRow)
		s.Require().NoError(err, "bridge node GetRow should succeed for valid row index")
		s.Require().NotNil(bridgeRow, "bridge node GetRow should return valid row")

		// Test light node GetRow with polling (bitswap-dependent)
		lightRow := s.pollGetRow(ctx, lightClient, height, testRow, 45*time.Second)
		s.Require().NotNil(lightRow, "light node GetRow should succeed for valid row index")
	})

	s.Run("GetRange", func() {
		// Get the blob that was submitted to determine its range
		submittedBlob, err := bridgeClient.Blob.Get(ctx, height, nodeBlobs[0].Namespace(), nodeBlobs[0].Commitment)
		s.Require().NoError(err, "should be able to get submitted blob")
		s.Require().NotNil(submittedBlob, "submitted blob should not be nil")

		// Get blob length to determine the range
		blobLength, err := submittedBlob.Length()
		s.Require().NoError(err, "should be able to get blob length")

		if blobLength == 0 {
			s.T().Skip("Skipping GetRange test - blob has no data (length = 0)")
		}

		// Use the blob's actual range (from its index to index + length)
		blobIndex := submittedBlob.Index()
		rangeStart := int(blobIndex)
		rangeEnd := int(blobIndex + blobLength)

		// Test bridge node first (always works)
		var bridgeRangeData *nodeshare.GetRangeResult
		bridgeRangeData, err = bridgeClient.Share.GetRange(ctx, height, rangeStart, rangeEnd)
		s.Require().NoError(err, "bridge node GetRange should succeed for valid range")
		s.Require().NotNil(bridgeRangeData, "bridge node GetRange should return valid range data")

		// Test light node GetRange with polling (bitswap-dependent)
		lightRangeData := s.pollGetRange(ctx, lightClient, height, rangeStart, rangeEnd, 45*time.Second)
		s.Require().NotNil(lightRangeData, "light node GetRange should succeed for valid range")
	})
}

// TestHeaderAPIContract validates Header module API responses
func (s *APITestSuite) TestHeaderAPIContract() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Create light node for testing
	lightNode := s.framework.NewLightNode(ctx)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Wait for more blocks to ensure nodes are fully synced
	_, err := bridgeClient.Header.WaitForHeight(ctx, 20)
	s.Require().NoError(err, "should have sufficient blocks available for testing")

	s.Run("LocalHead", func() {
		bridgeHead, err := bridgeClient.Header.LocalHead(ctx)
		s.Require().NoError(err, "bridge node LocalHead should succeed")
		s.Require().NotNil(bridgeHead, "bridge node LocalHead should return valid header")
		s.Require().Greater(bridgeHead.Height(), uint64(0), "bridge node LocalHead should return positive height")

		lightHead, err := lightClient.Header.LocalHead(ctx)
		s.Require().NoError(err, "light node LocalHead should succeed")
		s.Require().NotNil(lightHead, "light node LocalHead should return valid header")
		s.Require().Greater(lightHead.Height(), uint64(0), "light node LocalHead should return positive height")
	})

	s.Run("GetByHash", func() {
		header, err := bridgeClient.Header.GetByHeight(ctx, 5)
		s.Require().NoError(err, "should be able to get header by height")

		bridgeHeaderByHash, err := bridgeClient.Header.GetByHash(ctx, header.Hash())
		s.Require().NoError(err, "bridge node GetByHash should succeed for valid hash")
		s.Require().NotNil(bridgeHeaderByHash, "bridge node GetByHash should return valid header")
		s.Require().Equal(header.Hash(), bridgeHeaderByHash.Hash(), "bridge node GetByHash should return same header")

		lightHeaderByHash, err := lightClient.Header.GetByHash(ctx, header.Hash())
		s.Require().NoError(err, "light node GetByHash should succeed for valid hash")
		s.Require().NotNil(lightHeaderByHash, "light node GetByHash should return valid header")
		s.Require().Equal(header.Hash(), lightHeaderByHash.Hash(), "light node GetByHash should return same header")

	})

	s.Run("GetByHeight", func() {
		bridgeHeader, err := bridgeClient.Header.GetByHeight(ctx, 5)
		s.Require().NoError(err, "bridge node GetByHeight should succeed for valid height")
		s.Require().NotNil(bridgeHeader, "bridge node GetByHeight should return valid header")
		s.Require().Equal(uint64(5), bridgeHeader.Height(), "bridge node GetByHeight should return correct height")

		lightHeader, err := lightClient.Header.GetByHeight(ctx, 5)
		s.Require().NoError(err, "light node GetByHeight should succeed for valid height")
		s.Require().NotNil(lightHeader, "light node GetByHeight should return valid header")
		s.Require().Equal(uint64(5), lightHeader.Height(), "light node GetByHeight should return correct height")

	})

	s.Run("WaitForHeight", func() {
		bridgeHeader, err := bridgeClient.Header.WaitForHeight(ctx, 5)
		s.Require().NoError(err, "bridge node WaitForHeight should succeed for current height")
		s.Require().NotNil(bridgeHeader, "bridge node WaitForHeight should return valid header")

		lightHeader, err := lightClient.Header.WaitForHeight(ctx, 5)
		s.Require().NoError(err, "light node WaitForHeight should succeed for current height")
		s.Require().NotNil(lightHeader, "light node WaitForHeight should return valid header")

	})

	s.Run("GetRangeByHeight", func() {
		startHeader, err := bridgeClient.Header.GetByHeight(ctx, 1)
		s.Require().NoError(err, "should be able to get starting header")

		bridgeHeaders, err := bridgeClient.Header.GetRangeByHeight(ctx, startHeader, 3)
		s.Require().NoError(err, "bridge node GetRangeByHeight should succeed for valid range")
		s.Require().NotEmpty(bridgeHeaders, "bridge node GetRangeByHeight should return some headers")
		s.Require().LessOrEqual(len(bridgeHeaders), 3, "bridge node GetRangeByHeight should return at most requested number of headers")

		lightHeaders, err := lightClient.Header.GetRangeByHeight(ctx, startHeader, 3)
		s.Require().NoError(err, "light node GetRangeByHeight should succeed for valid range")
		s.Require().NotEmpty(lightHeaders, "light node GetRangeByHeight should return some headers")
		s.Require().LessOrEqual(len(lightHeaders), 3, "light node GetRangeByHeight should return at most requested number of headers")

	})

	s.Run("SyncState", func() {
		bridgeState, err := bridgeClient.Header.SyncState(ctx)
		s.Require().NoError(err, "bridge node SyncState should succeed")
		s.Require().NotNil(bridgeState, "bridge node SyncState should return valid state")

		lightState, err := lightClient.Header.SyncState(ctx)
		s.Require().NoError(err, "light node SyncState should succeed")
		s.Require().NotNil(lightState, "light node SyncState should return valid state")
	})

	s.Run("SyncWait", func() {
		err := bridgeClient.Header.SyncWait(ctx)
		s.Require().NoError(err, "bridge node SyncWait should succeed if synced")

		err = lightClient.Header.SyncWait(ctx)
		s.Require().NoError(err, "light node SyncWait should succeed if synced")
	})

	s.Run("NetworkHead", func() {
		bridgeHead, err := bridgeClient.Header.NetworkHead(ctx)
		s.Require().NoError(err, "bridge node NetworkHead should succeed")
		s.Require().NotNil(bridgeHead, "bridge node NetworkHead should return valid header")

		lightHead, err := lightClient.Header.NetworkHead(ctx)
		s.Require().NoError(err, "light node NetworkHead should succeed")
		s.Require().NotNil(lightHead, "light node NetworkHead should return valid header")
	})
}

// TestBlobAPIContract validates Blob module API responses
func (s *APITestSuite) TestBlobAPIContract() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]

	// Create a light node for testing
	lightNode := s.framework.NewLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x01}, 10))
	s.Require().NoError(err, "should create namespace")

	blobData := []byte("TestBlobAPIContract: API contract validation test data")
	nodeBlobs := s.createBlobsForSubmission(ctx, bridgeClient, namespace, blobData)

	// Submit blob for testing
	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	height, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "should be able to submit blob for API testing")
	s.Require().NotZero(height, "blob submission should return valid height")

	// Wait for inclusion
	_, err = bridgeClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "should wait for blob inclusion")

	// Test Submit API
	s.Run("Submit", func() {
		testNamespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x02}, 10))
		s.Require().NoError(err, "should create test namespace")

		testData := []byte("TestSubmitContract: Submit API validation")
		testBlobs := s.createBlobsForSubmission(ctx, bridgeClient, testNamespace, testData)

		submitHeight, err := bridgeClient.Blob.Submit(ctx, testBlobs, txConfig)
		s.Require().NoError(err, "bridge node Submit should succeed for valid blob")
		s.Require().NotZero(submitHeight, "bridge node Submit should return valid height")
	})

	// Test Get API
	s.Run("Get", func() {
		bridgeBlob, err := bridgeClient.Blob.Get(ctx, height, namespace, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "bridge node Get should succeed for valid parameters")
		s.Require().NotNil(bridgeBlob, "bridge node Get should return valid blob")

		lightBlob, err := lightClient.Blob.Get(ctx, height, namespace, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "light node Get should succeed for valid parameters")
		s.Require().NotNil(lightBlob, "light node Get should return valid blob")
	})

	s.Run("GetAll", func() {
		bridgeBlobs, err := bridgeClient.Blob.GetAll(ctx, height, []share.Namespace{namespace})
		s.Require().NoError(err, "bridge node GetAll should succeed for valid namespace")
		s.Require().NotEmpty(bridgeBlobs, "bridge node GetAll should return blobs for valid namespace")

		lightBlobs, err := lightClient.Blob.GetAll(ctx, height, []share.Namespace{namespace})
		s.Require().NoError(err, "light node GetAll should succeed for valid namespace")
		s.Require().NotEmpty(lightBlobs, "light node GetAll should return blobs for valid namespace")

		bridgeEmptyBlobs, err := bridgeClient.Blob.GetAll(ctx, height, []share.Namespace{})
		s.Require().NoError(err, "bridge node GetAll should succeed for empty namespace list")
		s.Require().Empty(bridgeEmptyBlobs, "bridge node GetAll should return empty for empty namespace list")

		lightEmptyBlobs, err := lightClient.Blob.GetAll(ctx, height, []share.Namespace{})
		s.Require().NoError(err, "light node GetAll should succeed for empty namespace list")
		s.Require().Empty(lightEmptyBlobs, "light node GetAll should return empty for empty namespace list")
	})

	s.Run("GetProof", func() {
		bridgeProof, err := bridgeClient.Blob.GetProof(ctx, height, namespace, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "bridge node GetProof should succeed for valid parameters")
		s.Require().NotNil(bridgeProof, "bridge node GetProof should return valid proof")

		lightProof, err := lightClient.Blob.GetProof(ctx, height, namespace, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "light node GetProof should succeed for valid parameters")
		s.Require().NotNil(lightProof, "light node GetProof should return valid proof")
	})

	s.Run("Included", func() {
		proof, err := bridgeClient.Blob.GetProof(ctx, height, namespace, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "should be able to get proof")

		bridgeIncluded, err := bridgeClient.Blob.Included(ctx, height, namespace, proof, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "bridge node Included should succeed for valid proof")
		s.Require().True(bridgeIncluded, "bridge node Included should return true for valid proof")

		lightIncluded, err := lightClient.Blob.Included(ctx, height, namespace, proof, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "light node Included should succeed for valid proof")
		s.Require().True(lightIncluded, "light node Included should return true for valid proof")
	})

	s.Run("GetCommitmentProof", func() {
		bridgeProof, err := bridgeClient.Blob.GetCommitmentProof(ctx, height, namespace, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "bridge node GetCommitmentProof should succeed for valid parameters")
		s.Require().NotNil(bridgeProof, "bridge node GetCommitmentProof should return valid proof")

		lightProof, err := lightClient.Blob.GetCommitmentProof(ctx, height, namespace, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "light node GetCommitmentProof should succeed for valid parameters")
		s.Require().NotNil(lightProof, "light node GetCommitmentProof should return valid proof")
	})
}

// TestStateAPIContract validates State module API responses
func (s *APITestSuite) TestStateAPIContract() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Create light node for testing
	lightNode := s.framework.NewLightNode(ctx)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	s.Run("AccountAddress", func() {
		bridgeAddr, err := bridgeClient.State.AccountAddress(ctx)
		s.Require().NoError(err, "bridge node AccountAddress should succeed")
		s.Require().NotNil(bridgeAddr, "bridge node AccountAddress should return valid address")
		s.Require().NotEmpty(bridgeAddr.String(), "bridge node AccountAddress should return non-empty string")

		lightAddr, err := lightClient.State.AccountAddress(ctx)
		s.Require().NoError(err, "light node AccountAddress should succeed")
		s.Require().NotNil(lightAddr, "light node AccountAddress should return valid address")
		s.Require().NotEmpty(lightAddr.String(), "light node AccountAddress should return non-empty string")
	})

	s.Run("Balance", func() {
		bridgeAddr, err := bridgeClient.State.AccountAddress(ctx)
		s.Require().NoError(err, "should be able to get bridge account address")

		lightAddr, err := lightClient.State.AccountAddress(ctx)
		s.Require().NoError(err, "should be able to get light account address")

		bridgeBalance, err := bridgeClient.State.Balance(ctx)
		s.Require().NoError(err, "bridge node Balance should succeed")
		s.Require().NotNil(bridgeBalance, "bridge node Balance should return valid balance")

		lightBalance, err := lightClient.State.Balance(ctx)
		s.Require().NoError(err, "light node Balance should succeed")
		s.Require().NotNil(lightBalance, "light node Balance should return valid balance")

		bridgeBalanceForAddr, err := bridgeClient.State.BalanceForAddress(ctx, bridgeAddr)
		s.Require().NoError(err, "bridge node BalanceForAddress should succeed for valid address")
		s.Require().NotNil(bridgeBalanceForAddr, "bridge node BalanceForAddress should return valid balance")

		lightBalanceForAddr, err := lightClient.State.BalanceForAddress(ctx, lightAddr)
		s.Require().NoError(err, "light node BalanceForAddress should succeed for valid address")
		s.Require().NotNil(lightBalanceForAddr, "light node BalanceForAddress should return valid balance")
	})

	s.Run("SubmitPayForBlob", func() {
		namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x03}, 10))
		s.Require().NoError(err, "should create namespace")

		blobData := []byte("TestSubmitPayForBlobContract: State API validation")
		bridgeLibBlobs := s.createLibshareBlobs(ctx, bridgeClient, namespace, blobData)

		txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
		bridgeHeight, err := bridgeClient.State.SubmitPayForBlob(ctx, bridgeLibBlobs, txConfig)
		s.Require().NoError(err, "bridge node SubmitPayForBlob should succeed for valid blobs")
		s.Require().NotZero(bridgeHeight, "bridge node SubmitPayForBlob should return valid height")

		lightLibBlobs := s.createLibshareBlobs(ctx, lightClient, namespace, blobData)
		lightHeight, err := lightClient.State.SubmitPayForBlob(ctx, lightLibBlobs, txConfig)
		s.Require().NoError(err, "light node SubmitPayForBlob should succeed for valid blobs")
		s.Require().NotZero(lightHeight, "light node SubmitPayForBlob should return valid height")
	})
}

// TestP2PAPIContract validates P2P module API responses
func (s *APITestSuite) TestP2PAPIContract() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get nodes
	bridgeNode := s.framework.GetBridgeNodes()[0]
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Create light node for testing
	lightNode := s.framework.NewLightNode(ctx)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	s.Run("Info", func() {
		bridgeInfo, err := bridgeClient.P2P.Info(ctx)
		s.Require().NoError(err, "bridge node P2P Info should succeed")
		s.Require().NotNil(bridgeInfo, "bridge node P2P Info should return valid info")

		lightInfo, err := lightClient.P2P.Info(ctx)
		s.Require().NoError(err, "light node P2P Info should succeed")
		s.Require().NotNil(lightInfo, "light node P2P Info should return valid info")
	})

	s.Run("Peers", func() {
		bridgePeers, err := bridgeClient.P2P.Peers(ctx)
		s.Require().NoError(err, "bridge node P2P Peers should succeed")
		s.Require().NotNil(bridgePeers, "bridge node P2P Peers should return valid peers")

		lightPeers, err := lightClient.P2P.Peers(ctx)
		s.Require().NoError(err, "light node P2P Peers should succeed")
		s.Require().NotNil(lightPeers, "light node P2P Peers should return valid peers")
	})

	s.Run("PeerInfo", func() {
		bridgeInfo, err := bridgeClient.P2P.Info(ctx)
		s.Require().NoError(err, "should be able to get bridge info")

		bridgePeerInfo, err := lightClient.P2P.PeerInfo(ctx, bridgeInfo.ID)
		s.Require().NoError(err, "light node PeerInfo should succeed for valid peer ID")
		s.Require().NotNil(bridgePeerInfo, "light node PeerInfo should return valid peer info")

		lightInfo, err := lightClient.P2P.Info(ctx)
		s.Require().NoError(err, "should be able to get light info")

		lightPeerInfo, err := bridgeClient.P2P.PeerInfo(ctx, lightInfo.ID)
		s.Require().NoError(err, "bridge node PeerInfo should succeed for valid peer ID")
		s.Require().NotNil(lightPeerInfo, "bridge node PeerInfo should return valid peer info")
	})

	s.Run("Connectedness", func() {
		bridgeInfo, err := bridgeClient.P2P.Info(ctx)
		s.Require().NoError(err, "should be able to get bridge info")

		bridgeConnectedness, err := lightClient.P2P.Connectedness(ctx, bridgeInfo.ID)
		s.Require().NoError(err, "light node Connectedness should succeed for valid peer ID")
		s.Require().NotNil(bridgeConnectedness, "light node Connectedness should return valid status")

		lightInfo, err := lightClient.P2P.Info(ctx)
		s.Require().NoError(err, "should be able to get light info")

		lightConnectedness, err := bridgeClient.P2P.Connectedness(ctx, lightInfo.ID)
		s.Require().NoError(err, "bridge node Connectedness should succeed for valid peer ID")
		s.Require().NotNil(lightConnectedness, "bridge node Connectedness should return valid status")
	})
}

// TestNodeAPIContract validates Node module API responses
func (s *APITestSuite) TestNodeAPIContract() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Create light node for testing
	lightNode := s.framework.NewLightNode(ctx)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	s.Run("Info", func() {
		bridgeInfo, err := bridgeClient.Node.Info(ctx)
		s.Require().NoError(err, "bridge node Info should succeed")
		s.Require().NotNil(bridgeInfo, "bridge node Info should return valid info")

		lightInfo, err := lightClient.Node.Info(ctx)
		s.Require().NoError(err, "light node Info should succeed")
		s.Require().NotNil(lightInfo, "light node Info should return valid info")
	})

	s.Run("Ready", func() {
		bridgeReady, err := bridgeClient.Node.Ready(ctx)
		s.Require().NoError(err, "bridge node Ready should succeed")
		s.Require().True(bridgeReady, "bridge node should be ready")

		lightReady, err := lightClient.Node.Ready(ctx)
		s.Require().NoError(err, "light node Ready should succeed")
		s.Require().True(lightReady, "light node should be ready")
	})

	s.Run("AuthNew", func() {
		bridgeToken, err := bridgeClient.Node.AuthNew(ctx, []auth.Permission{"public", "read"})
		s.Require().NoError(err, "bridge node AuthNew should succeed for valid permissions")
		s.Require().NotEmpty(bridgeToken, "bridge node AuthNew should return non-empty token")

		lightToken, err := lightClient.Node.AuthNew(ctx, []auth.Permission{"public", "read"})
		s.Require().NoError(err, "light node AuthNew should succeed for valid permissions")
		s.Require().NotEmpty(lightToken, "light node AuthNew should return non-empty token")
	})

	s.Run("AuthNewWithExpiry", func() {
		bridgeToken, err := bridgeClient.Node.AuthNewWithExpiry(ctx, []auth.Permission{"public", "read"}, time.Hour)
		s.Require().NoError(err, "bridge node AuthNewWithExpiry should succeed for valid parameters")
		s.Require().NotEmpty(bridgeToken, "bridge node AuthNewWithExpiry should return non-empty token")

		lightToken, err := lightClient.Node.AuthNewWithExpiry(ctx, []auth.Permission{"public", "read"}, time.Hour)
		s.Require().NoError(err, "light node AuthNewWithExpiry should succeed for valid parameters")
		s.Require().NotEmpty(lightToken, "light node AuthNewWithExpiry should return non-empty token")
	})
}

// TestBlobstreamAPIContract validates Blobstream module API responses
func (s *APITestSuite) TestBlobstreamAPIContract() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Create light node for testing
	lightNode := s.framework.NewLightNode(ctx)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	_, err := bridgeClient.Header.WaitForHeight(ctx, 5)
	s.Require().NoError(err, "should have blocks available for testing")

	s.Run("GetDataRootTupleRoot", func() {
		bridgeTupleRoot, err := bridgeClient.Blobstream.GetDataRootTupleRoot(ctx, 1, 5)
		s.Require().NoError(err, "bridge node GetDataRootTupleRoot should succeed for valid height range")
		s.Require().NotNil(bridgeTupleRoot, "bridge node GetDataRootTupleRoot should return valid tuple root")

		lightTupleRoot, err := lightClient.Blobstream.GetDataRootTupleRoot(ctx, 1, 5)
		s.Require().NoError(err, "light node GetDataRootTupleRoot should succeed for valid height range")
		s.Require().NotNil(lightTupleRoot, "light node GetDataRootTupleRoot should return valid tuple root")

	})

	s.Run("GetDataRootTupleInclusionProof", func() {
		bridgeProof, err := bridgeClient.Blobstream.GetDataRootTupleInclusionProof(ctx, 3, 1, 5)
		s.Require().NoError(err, "bridge node GetDataRootTupleInclusionProof should succeed for valid parameters")
		s.Require().NotNil(bridgeProof, "bridge node GetDataRootTupleInclusionProof should return valid proof")

		lightProof, err := lightClient.Blobstream.GetDataRootTupleInclusionProof(ctx, 3, 1, 5)
		s.Require().NoError(err, "light node GetDataRootTupleInclusionProof should succeed for valid parameters")
		s.Require().NotNil(lightProof, "light node GetDataRootTupleInclusionProof should return valid proof")

	})
}

func (s *APITestSuite) createBlobsForSubmission(ctx context.Context, client *rpcclient.Client, namespace share.Namespace, data []byte) []*nodeblob.Blob {
	nodeAddr, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err, "should get node address")

	libBlob, err := share.NewV1Blob(namespace, data, nodeAddr.Bytes())
	s.Require().NoError(err, "should create libshare blob")

	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlob)
	s.Require().NoError(err, "should convert to node blobs")

	return nodeBlobs
}

func (s *APITestSuite) createLibshareBlobs(ctx context.Context, client *rpcclient.Client, namespace share.Namespace, data []byte) []*share.Blob {
	nodeAddr, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err, "should get node address")

	libBlob, err := share.NewV1Blob(namespace, data, nodeAddr.Bytes())
	s.Require().NoError(err, "should create libshare blob")

	return []*share.Blob{libBlob}
}

// pollSharesAvailable polls for shares availability with retry logic and returns true if successful
func (s *APITestSuite) pollSharesAvailable(ctx context.Context, client *rpcclient.Client, height uint64, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	retryInterval := 1 * time.Second
	requestTimeout := 15 * time.Second

	for {
		reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		err := client.Share.SharesAvailable(reqCtx, height)
		cancel()

		if err == nil {
			return true
		}

		if time.Now().After(deadline) {
			return false
		}

		time.Sleep(retryInterval)
	}
}

// pollGetSamples polls for GetSamples with retry logic and returns the samples if successful
func (s *APITestSuite) pollGetSamples(ctx context.Context, client *rpcclient.Client, height uint64, coords []shwap.SampleCoords, timeout time.Duration) []shwap.Sample {
	deadline := time.Now().Add(timeout)
	retryInterval := 1 * time.Second
	requestTimeout := 10 * time.Second

	for {
		reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		samples, err := client.Share.GetSamples(reqCtx, height, coords)
		cancel()

		if err == nil {
			return samples
		}

		if time.Now().After(deadline) {
			return nil
		}

		time.Sleep(retryInterval)
	}
}

// pollGetRow polls for GetRow with retry logic and returns the row if successful
func (s *APITestSuite) pollGetRow(ctx context.Context, client *rpcclient.Client, height uint64, row int, timeout time.Duration) shwap.Row {
	deadline := time.Now().Add(timeout)
	retryInterval := 1 * time.Second
	requestTimeout := 10 * time.Second

	for {
		reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		rowData, err := client.Share.GetRow(reqCtx, height, row)
		cancel()

		if err == nil {
			return rowData
		}

		if time.Now().After(deadline) {
			return shwap.Row{}
		}

		time.Sleep(retryInterval)
	}
}

// pollGetRange polls for GetRange with retry logic and returns the range data if successful
func (s *APITestSuite) pollGetRange(ctx context.Context, client *rpcclient.Client, height uint64, start, end int, timeout time.Duration) *nodeshare.GetRangeResult {
	deadline := time.Now().Add(timeout)
	retryInterval := 1 * time.Second
	requestTimeout := 10 * time.Second

	for {
		reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		rangeData, err := client.Share.GetRange(reqCtx, height, start, end)
		cancel()

		if err == nil {
			return rangeData
		}

		if time.Now().After(deadline) {
			return nil
		}

		time.Sleep(retryInterval)
	}
}
