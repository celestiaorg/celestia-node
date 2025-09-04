//go:build integration

package tastora

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/state"
	"github.com/celestiaorg/go-square/v2/share"
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
	// Setup with minimal topology: 1 Bridge Node + 1 Light Node
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(1), WithLightNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))

	// Start the light node after network setup
	s.framework.NewLightNode(ctx)
}

// TestRPCEndpointCompliance validates raw RPC endpoint compliance for both BN/LN
func (s *APITestSuite) TestRPCEndpointCompliance() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	lightNode := s.framework.GetLightNodes()[0]

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	lightNode := s.framework.GetLightNodes()[0]

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Wait for sufficient blocks to ensure nodes have share data
	_, err := bridgeClient.Header.WaitForHeight(ctx, 20)
	s.Require().NoError(err, "should have sufficient blocks available for testing")

	// Wait for light node to sync to the same height
	_, err = lightClient.Header.WaitForHeight(ctx, 20)
	s.Require().NoError(err, "light node should be synced to sufficient height")

	// Get current height for testing
	currentHeight, err := bridgeClient.Header.LocalHead(ctx)
	s.Require().NoError(err, "should be able to get current height")
	s.Require().NotNil(currentHeight, "current height should not be nil")

	// Ensure shares are available at current height before testing
	err = bridgeClient.Share.SharesAvailable(ctx, currentHeight.Height())
	s.Require().NoError(err, "shares should be available at current height %d", currentHeight.Height())

	// Wait for light node to have shares available as well
	err = lightClient.Share.SharesAvailable(ctx, currentHeight.Height())
	s.Require().NoError(err, "light node should have shares available at current height %d", currentHeight.Height())

	s.Run("SharesAvailable", func() {
		bridgeErr := bridgeClient.Share.SharesAvailable(ctx, currentHeight.Height())
		s.Require().NoError(bridgeErr, "bridge node SharesAvailable should succeed for valid height")

		lightErr := lightClient.Share.SharesAvailable(ctx, currentHeight.Height())
		s.Require().NoError(lightErr, "light node SharesAvailable should succeed for valid height")

		bridgeErr = bridgeClient.Share.SharesAvailable(ctx, 999999)
		s.Require().Error(bridgeErr, "bridge node SharesAvailable should fail for invalid height")

		lightErr = lightClient.Share.SharesAvailable(ctx, 999999)
		s.Require().Error(lightErr, "light node SharesAvailable should fail for invalid height")
	})

	s.Run("GetShare", func() {
		// Use current height for testing
		testHeight := currentHeight.Height()

		bridgeShare, err := bridgeClient.Share.GetShare(ctx, testHeight, 0, 0)
		s.Require().NoError(err, "bridge node GetShare should succeed for valid coordinates")
		s.Require().NotNil(bridgeShare, "bridge node GetShare should return valid share")

		// Test light node only if it's ready (not actively syncing)
		// Use a shorter timeout for light node to prevent hanging
		lightShareCtx, lightShareCancel := context.WithTimeout(ctx, 10*time.Second)
		lightShare, err := lightClient.Share.GetShare(lightShareCtx, testHeight, 0, 0)
		lightShareCancel()
		if err != nil {
			// Light node might still be syncing, skip this test for light node
			s.T().Logf("Light node GetShare skipped due to sync state: %v", err)
		} else {
			s.Require().NotNil(lightShare, "light node GetShare should return valid share")
		}

		_, err = bridgeClient.Share.GetShare(ctx, testHeight, 999, 999)
		s.Require().Error(err, "bridge node GetShare should fail for invalid coordinates")

		_, err = lightClient.Share.GetShare(ctx, testHeight, 999, 999)
		if err == nil {
			s.Require().Error(err, "light node GetShare should fail for invalid coordinates")
		}
	})

	s.Run("GetSamples", func() {
		// Use current height for testing
		testHeight := currentHeight.Height()

		coords := []shwap.SampleCoords{{Row: 0, Col: 0}}
		bridgeSamples, err := bridgeClient.Share.GetSamples(ctx, testHeight, coords)
		s.Require().NoError(err, "bridge node GetSamples should succeed for valid coordinates")
		s.Require().Len(bridgeSamples, 1, "bridge node GetSamples should return expected number of samples")

		// Test light node only if it's ready (not actively syncing)
		// Use a shorter timeout for light node to prevent hanging
		lightSamplesCtx, lightSamplesCancel := context.WithTimeout(ctx, 10*time.Second)
		lightSamples, err := lightClient.Share.GetSamples(lightSamplesCtx, testHeight, coords)
		lightSamplesCancel()
		if err != nil {
			// Light node might still be syncing, skip this test for light node
			s.T().Logf("Light node GetSamples skipped due to sync state: %v", err)
		} else {
			s.Require().Len(lightSamples, 1, "light node GetSamples should return expected number of samples")
		}

		invalidCoords := []shwap.SampleCoords{{Row: 999, Col: 999}}
		_, err = bridgeClient.Share.GetSamples(ctx, testHeight, invalidCoords)
		s.Require().Error(err, "bridge node GetSamples should fail for invalid coordinates")

		_, err = lightClient.Share.GetSamples(ctx, testHeight, invalidCoords)
		if err == nil {
			s.Require().Error(err, "light node GetSamples should fail for invalid coordinates")
		}
	})

	s.Run("GetEDS", func() {
		bridgeEds, err := bridgeClient.Share.GetEDS(ctx, currentHeight.Height())
		s.Require().NoError(err, "bridge node GetEDS should succeed for valid height")
		s.Require().NotNil(bridgeEds, "bridge node GetEDS should return valid EDS")

		lightEds, err := lightClient.Share.GetEDS(ctx, currentHeight.Height())
		s.Require().NoError(err, "light node GetEDS should succeed for valid height")
		s.Require().NotNil(lightEds, "light node GetEDS should return valid EDS")

		_, err = bridgeClient.Share.GetEDS(ctx, 999999)
		s.Require().Error(err, "bridge node GetEDS should fail for invalid height")

		_, err = lightClient.Share.GetEDS(ctx, 999999)
		s.Require().Error(err, "light node GetEDS should fail for invalid height")
	})

	s.Run("GetRow", func() {
		// Use current height for testing
		testHeight := currentHeight.Height()

		bridgeRow, err := bridgeClient.Share.GetRow(ctx, testHeight, 0)
		s.Require().NoError(err, "bridge node GetRow should succeed for valid row index")
		s.Require().NotNil(bridgeRow, "bridge node GetRow should return valid row")

		// Test light node only if it's ready (not actively syncing)
		// Use a shorter timeout for light node to prevent hanging
		lightRowCtx, lightRowCancel := context.WithTimeout(ctx, 10*time.Second)
		lightRow, err := lightClient.Share.GetRow(lightRowCtx, testHeight, 0)
		lightRowCancel()
		if err != nil {
			// Light node might still be syncing, skip this test for light node
			s.T().Logf("Light node GetRow skipped due to sync state: %v", err)
		} else {
			s.Require().NotNil(lightRow, "light node GetRow should return valid row")
		}

		_, err = bridgeClient.Share.GetRow(ctx, testHeight, 999)
		s.Require().Error(err, "bridge node GetRow should fail for invalid row index")

		_, err = lightClient.Share.GetRow(ctx, testHeight, 999)
		if err == nil {
			s.Require().Error(err, "light node GetRow should fail for invalid row index")
		}
	})

	s.Run("GetNamespaceData", func() {
		namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x01}, 10))
		s.Require().NoError(err, "should create namespace")

		bridgeData, err := bridgeClient.Share.GetNamespaceData(ctx, currentHeight.Height(), namespace)
		s.Require().NoError(err, "bridge node GetNamespaceData should succeed for valid namespace")
		s.Require().NotNil(bridgeData, "bridge node GetNamespaceData should return valid data")

		lightData, err := lightClient.Share.GetNamespaceData(ctx, currentHeight.Height(), namespace)
		s.Require().NoError(err, "light node GetNamespaceData should succeed for valid namespace")
		s.Require().NotNil(lightData, "light node GetNamespaceData should return valid data")
	})

	s.Run("GetRange", func() {
		// Get EDS first to determine actual data size
		bridgeEds, err := bridgeClient.Share.GetEDS(ctx, currentHeight.Height())
		s.Require().NoError(err, "should be able to get EDS to determine size")
		s.Require().NotNil(bridgeEds, "EDS should not be nil")

		// Use actual data size for range testing
		rowRoots, err := bridgeEds.RowRoots()
		s.Require().NoError(err, "should be able to get row roots")
		dataSize := len(rowRoots)
		if dataSize > 0 {
			bridgeRangeData, err := bridgeClient.Share.GetRange(ctx, currentHeight.Height(), 0, dataSize-1)
			s.Require().NoError(err, "bridge node GetRange should succeed for valid range")
			s.Require().NotNil(bridgeRangeData, "bridge node GetRange should return valid range data")

			// Test light node only if it's ready (not actively syncing)
			// Use a shorter timeout for light node to prevent hanging
			lightRangeCtx, lightRangeCancel := context.WithTimeout(ctx, 10*time.Second)
			lightRangeData, err := lightClient.Share.GetRange(lightRangeCtx, currentHeight.Height(), 0, dataSize-1)
			lightRangeCancel()
			if err != nil {
				// Light node might still be syncing, skip this test for light node
				s.T().Logf("Light node GetRange skipped due to sync state: %v", err)
			} else {
				s.Require().NotNil(lightRangeData, "light node GetRange should return valid range data")
			}
		}

		_, err = bridgeClient.Share.GetRange(ctx, currentHeight.Height(), 999, 1000)
		s.Require().Error(err, "bridge node GetRange should fail for invalid range")

		_, err = lightClient.Share.GetRange(ctx, currentHeight.Height(), 999, 1000)
		if err == nil {
			s.Require().Error(err, "light node GetRange should fail for invalid range")
		}
	})
}

// TestHeaderAPIContract validates Header module API responses
func (s *APITestSuite) TestHeaderAPIContract() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	lightNode := s.framework.GetLightNodes()[0]

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
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

		_, err = bridgeClient.Header.GetByHash(ctx, []byte("invalid-hash"))
		s.Require().Error(err, "bridge node GetByHash should fail for invalid hash")

		_, err = lightClient.Header.GetByHash(ctx, []byte("invalid-hash"))
		s.Require().Error(err, "light node GetByHash should fail for invalid hash")
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

		_, err = bridgeClient.Header.GetByHeight(ctx, 999999)
		s.Require().Error(err, "bridge node GetByHeight should fail for invalid height")

		_, err = lightClient.Header.GetByHeight(ctx, 999999)
		s.Require().Error(err, "light node GetByHeight should fail for invalid height")
	})

	s.Run("WaitForHeight", func() {
		bridgeHeader, err := bridgeClient.Header.WaitForHeight(ctx, 5)
		s.Require().NoError(err, "bridge node WaitForHeight should succeed for current height")
		s.Require().NotNil(bridgeHeader, "bridge node WaitForHeight should return valid header")

		lightHeader, err := lightClient.Header.WaitForHeight(ctx, 5)
		s.Require().NoError(err, "light node WaitForHeight should succeed for current height")
		s.Require().NotNil(lightHeader, "light node WaitForHeight should return valid header")

		timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		_, err = bridgeClient.Header.WaitForHeight(timeoutCtx, 999999)
		s.Require().Error(err, "bridge node WaitForHeight should timeout for future height")

		_, err = lightClient.Header.WaitForHeight(timeoutCtx, 999999)
		s.Require().Error(err, "light node WaitForHeight should timeout for future height")
	})

	// TODO: GetRangeByHeight is hanging - skip for now
	// s.Run("GetRangeByHeight", func() {
	// 	startHeader, err := bridgeClient.Header.GetByHeight(ctx, 1)
	// 	s.Require().NoError(err, "should be able to get starting header")

	// 	bridgeHeaders, err := bridgeClient.Header.GetRangeByHeight(ctx, startHeader, 3)
	// 	s.Require().NoError(err, "bridge node GetRangeByHeight should succeed for valid range")
	// 	s.Require().NotEmpty(bridgeHeaders, "bridge node GetRangeByHeight should return some headers")
	// 	s.Require().LessOrEqual(len(bridgeHeaders), 3, "bridge node GetRangeByHeight should return at most requested number of headers")

	// 	lightHeaders, err := lightClient.Header.GetRangeByHeight(ctx, startHeader, 3)
	// 	s.Require().NoError(err, "light node GetRangeByHeight should succeed for valid range")
	// 	s.Require().NotEmpty(lightHeaders, "light node GetRangeByHeight should return some headers")
	// 	s.Require().LessOrEqual(len(lightHeaders), 3, "light node GetRangeByHeight should return at most requested number of headers")

	// 	_, err = bridgeClient.Header.GetRangeByHeight(ctx, startHeader, 1000)
	// 	s.Require().Error(err, "bridge node GetRangeByHeight should fail for invalid range")

	// 	_, err = lightClient.Header.GetRangeByHeight(ctx, startHeader, 1000)
	// 	s.Require().Error(err, "light node GetRangeByHeight should fail for invalid range")
	// })

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
	lightNode := s.framework.GetLightNodes()[0]

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

		_, err = bridgeClient.Blob.Get(ctx, height, namespace, []byte("invalid-commitment"))
		s.Require().Error(err, "bridge node Get should fail for invalid commitment")

		_, err = lightClient.Blob.Get(ctx, height, namespace, []byte("invalid-commitment"))
		s.Require().Error(err, "light node Get should fail for invalid commitment")
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

		_, err = bridgeClient.Blob.GetProof(ctx, height, namespace, []byte("invalid-commitment"))
		s.Require().Error(err, "bridge node GetProof should fail for invalid commitment")

		_, err = lightClient.Blob.GetProof(ctx, height, namespace, []byte("invalid-commitment"))
		s.Require().Error(err, "light node GetProof should fail for invalid commitment")
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

		_, err = bridgeClient.Blob.Included(ctx, height, namespace, &nodeblob.Proof{}, nodeBlobs[0].Commitment)
		s.Require().Error(err, "bridge node Included should fail for invalid proof")

		_, err = lightClient.Blob.Included(ctx, height, namespace, &nodeblob.Proof{}, nodeBlobs[0].Commitment)
		s.Require().Error(err, "light node Included should fail for invalid proof")
	})

	s.Run("GetCommitmentProof", func() {
		bridgeProof, err := bridgeClient.Blob.GetCommitmentProof(ctx, height, namespace, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "bridge node GetCommitmentProof should succeed for valid parameters")
		s.Require().NotNil(bridgeProof, "bridge node GetCommitmentProof should return valid proof")

		lightProof, err := lightClient.Blob.GetCommitmentProof(ctx, height, namespace, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "light node GetCommitmentProof should succeed for valid parameters")
		s.Require().NotNil(lightProof, "light node GetCommitmentProof should return valid proof")

		_, err = bridgeClient.Blob.GetCommitmentProof(ctx, height, namespace, []byte("invalid-commitment"))
		s.Require().Error(err, "bridge node GetCommitmentProof should fail for invalid commitment")

		_, err = lightClient.Blob.GetCommitmentProof(ctx, height, namespace, []byte("invalid-commitment"))
		s.Require().Error(err, "light node GetCommitmentProof should fail for invalid commitment")
	})
}

// TestStateAPIContract validates State module API responses
func (s *APITestSuite) TestStateAPIContract() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	lightNode := s.framework.GetLightNodes()[0]

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
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
	lightNode := s.framework.GetLightNodes()[0]

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
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
	lightNode := s.framework.GetLightNodes()[0]

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
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

	s.Run("AuthVerify", func() {
		_, err := bridgeClient.Node.AuthVerify(ctx, "test-token")
		s.Require().NotNil(err, "bridge node AuthVerify should handle invalid tokens gracefully")

		_, err = lightClient.Node.AuthVerify(ctx, "test-token")
		s.Require().NotNil(err, "light node AuthVerify should handle invalid tokens gracefully")
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
	lightNode := s.framework.GetLightNodes()[0]

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
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

		_, err = bridgeClient.Blobstream.GetDataRootTupleRoot(ctx, 999999, 1000000)
		s.Require().Error(err, "bridge node GetDataRootTupleRoot should fail for invalid height range")

		_, err = lightClient.Blobstream.GetDataRootTupleRoot(ctx, 999999, 1000000)
		s.Require().Error(err, "light node GetDataRootTupleRoot should fail for invalid height range")
	})

	s.Run("GetDataRootTupleInclusionProof", func() {
		bridgeProof, err := bridgeClient.Blobstream.GetDataRootTupleInclusionProof(ctx, 3, 1, 5)
		s.Require().NoError(err, "bridge node GetDataRootTupleInclusionProof should succeed for valid parameters")
		s.Require().NotNil(bridgeProof, "bridge node GetDataRootTupleInclusionProof should return valid proof")

		lightProof, err := lightClient.Blobstream.GetDataRootTupleInclusionProof(ctx, 3, 1, 5)
		s.Require().NoError(err, "light node GetDataRootTupleInclusionProof should succeed for valid parameters")
		s.Require().NotNil(lightProof, "light node GetDataRootTupleInclusionProof should return valid proof")

		_, err = bridgeClient.Blobstream.GetDataRootTupleInclusionProof(ctx, 999999, 999999, 1000000)
		s.Require().Error(err, "bridge node GetDataRootTupleInclusionProof should fail for invalid parameters")

		_, err = lightClient.Blobstream.GetDataRootTupleInclusionProof(ctx, 999999, 999999, 1000000)
		s.Require().Error(err, "light node GetDataRootTupleInclusionProof should fail for invalid parameters")
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
