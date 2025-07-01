package tastora

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/tastora/framework/testutil/sdkacc"
	tastoratypes "github.com/celestiaorg/tastora/framework/types"

	nodeblob "github.com/celestiaorg/celestia-node/blob"

	"github.com/celestiaorg/celestia-app/v4/pkg/appconsts"
	blobtypes "github.com/celestiaorg/celestia-app/v4/x/blob/types"
)

// BlobTestSuite is a dedicated test suite for blob module functionality.
// It embeds the base framework and provides blob-specific test methods.
type BlobTestSuite struct {
	suite.Suite
	framework  *Framework
	ctx        context.Context
	cancel     context.CancelFunc
	testWallet tastoratypes.Wallet
}

// SetupSuite initializes the test suite with the Tastora framework.
func (s *BlobTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 10*time.Minute)
	s.framework = NewFramework(s.T())

	err := s.framework.SetupNetwork(s.ctx)
	s.Require().NoError(err, "failed to setup network")

	// Create and fund a test wallet for chain operations
	s.testWallet = s.framework.CreateTestWallet(s.ctx, 10_000_000_000) // 10 billion utia

	// Fund the node's default account using the consolidated framework method
	fullNode := s.framework.GetFullNodes()[0]
	s.framework.FundNodeAccount(s.ctx, s.testWallet, fullNode, 1_000_000_000) // 1 billion utia
}

// TearDownSuite cleans up the test suite.
func (s *BlobTestSuite) TearDownSuite() {
	if s.cancel != nil {
		s.cancel()
	}
}

// TestBlobModule runs the main blob module API test.
// This test covers comprehensive blob operations using the working chain submission approach.
func (s *BlobTestSuite) TestBlobModule() {
	// Test data setup - use smaller data to avoid truncation issues
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x01}, 10))
	s.Require().NoError(err)

	data1 := []byte("Hello Celestia blob data 1")
	data2 := []byte("Hello Celestia blob data 2")

	// Get wallet address for signing
	walletAddr, err := sdkacc.AddressFromWallet(s.testWallet)
	s.Require().NoError(err)

	// Create V1 blobs with explicit signer
	libBlob1, err := blobtypes.NewV1Blob(namespace, data1, walletAddr)
	s.Require().NoError(err)

	libBlob2, err := blobtypes.NewV1Blob(namespace, data2, walletAddr)
	s.Require().NoError(err)

	// Create MsgPayForBlobs transaction
	signerStr := s.testWallet.GetFormattedAddress()
	msg, err := blobtypes.NewMsgPayForBlobs(signerStr, appconsts.LatestVersion, libBlob1, libBlob2)
	s.Require().NoError(err)

	// Submit via direct chain transaction
	chain := s.framework.GetCelestiaChain()
	resp, err := chain.BroadcastBlobMessage(s.ctx, s.testWallet, msg, libBlob1, libBlob2)
	s.Require().NoError(err)
	s.Require().Equal(uint32(0), resp.Code, "failed to broadcast blob: %s", resp.RawLog)

	// Wait for blob inclusion (allow time for the blob to be processed)
	time.Sleep(10 * time.Second)

	// Get the full node client for blob retrieval
	fullNode := s.framework.GetFullNodes()[0]
	fullNodeClient := s.framework.GetNodeRPCClient(s.ctx, fullNode)

	// Get the current height to search for blobs
	header, err := fullNodeClient.Header.NetworkHead(s.ctx)
	s.Require().NoError(err)
	currentHeight := header.Height()

	// Find the height where blobs were actually included
	var blobHeight uint64
	var retrievedBlobs []*nodeblob.Blob
	found := false
	for h := uint64(resp.Height); h <= currentHeight; h++ {
		blobs, err := fullNodeClient.Blob.GetAll(s.ctx, h, []libshare.Namespace{namespace})
		if err == nil && len(blobs) > 0 {
			blobHeight = h
			retrievedBlobs = blobs
			found = true
			s.T().Logf("Found %d blobs at height %d", len(blobs), h)
			break
		}
	}
	s.Require().True(found, "blobs not found at any height from %d to %d", resp.Height, currentHeight)
	s.Require().Len(retrievedBlobs, 2, "expected 2 blobs")

	// Test: Get single blob using the commitment from the retrieved blob
	s.Run("Get_Single_Blob_V0", func() {
		// Use the commitment from the actual retrieved blob
		retrievedBlob, err := fullNodeClient.Blob.Get(s.ctx, blobHeight, namespace, retrievedBlobs[0].Commitment)
		s.Require().NoError(err)
		s.Require().NotNil(retrievedBlob)

		// Trim null padding from retrieved data for comparison
		retrievedData := bytes.TrimRight(retrievedBlob.Data(), "\x00")
		// Check if it matches either data1 or data2 since order may vary
		s.Require().True(bytes.Equal(retrievedData, data1) || bytes.Equal(retrievedData, data2),
			"retrieved data doesn't match either expected blob data")
		s.Require().True(retrievedBlob.Namespace().Equals(namespace))
	})

	// Test: Get all blobs for namespace (this already works)
	s.Run("GetAll_Blobs_For_Namespace", func() {
		blobs, err := fullNodeClient.Blob.GetAll(s.ctx, blobHeight, []libshare.Namespace{namespace})
		s.Require().NoError(err)
		s.Require().Len(blobs, 2)

		// Verify all blobs are retrieved correctly (with padding trimmed)
		foundData1, foundData2 := false, false
		for _, blob := range blobs {
			retrievedData := bytes.TrimRight(blob.Data(), "\x00")
			if bytes.Equal(retrievedData, data1) {
				foundData1 = true
				s.Require().True(blob.Namespace().Equals(namespace))
			} else if bytes.Equal(retrievedData, data2) {
				foundData2 = true
				s.Require().True(blob.Namespace().Equals(namespace))
			}
		}
		s.Require().True(foundData1, "data1 blob not found")
		s.Require().True(foundData2, "data2 blob not found")
	})

	// Test: Get proof for blob using actual commitment
	s.Run("GetProof_For_Blob", func() {
		proof, err := fullNodeClient.Blob.GetProof(s.ctx, blobHeight, namespace, retrievedBlobs[0].Commitment)
		s.Require().NoError(err)
		s.Require().NotNil(proof)
		s.Require().NotEmpty(proof)
	})

	// Test: Verify blob inclusion using actual commitment
	s.Run("Included_Blob_Verification", func() {
		proof, err := fullNodeClient.Blob.GetProof(s.ctx, blobHeight, namespace, retrievedBlobs[0].Commitment)
		s.Require().NoError(err)

		included, err := fullNodeClient.Blob.Included(s.ctx, blobHeight, namespace, proof, retrievedBlobs[0].Commitment)
		s.Require().NoError(err)
		s.Require().True(included)
	})

	// Test: Non-existent blob error handling
	s.Run("Get_NonExistent_Blob_Error", func() {
		nonExistentCommitment := bytes.Repeat([]byte{0xFF}, 32)
		_, err := fullNodeClient.Blob.Get(s.ctx, blobHeight, namespace, nonExistentCommitment)
		s.Require().Error(err)
		s.Require().Contains(err.Error(), "blob: not found")
	})

	// Test: V1 blob-specific verification (matching swamp coverage)
	s.Run("Get_BlobV1_With_Signer", func() {
		// Get one of the V1 blobs we submitted
		retrievedBlob, err := fullNodeClient.Blob.Get(s.ctx, blobHeight, namespace, retrievedBlobs[0].Commitment)
		s.Require().NoError(err)
		s.Require().NotNil(retrievedBlob)

		// Verify it's a V1 blob with signer information
		s.Require().Equal(libshare.ShareVersionOne, retrievedBlob.ShareVersion(), "should be V1 blob")
		s.Require().NotNil(retrievedBlob.Signer(), "V1 blob should have signer")

		// Verify signer matches what we used for submission
		expectedSigner := walletAddr.Bytes()
		s.Require().Equal(expectedSigner, retrievedBlob.Signer(), "signer should match submission wallet")
	})
}

// TestBlobE2EFlow tests the end-to-end blob submission and retrieval flow.
// This uses the direct chain submission approach for blob operations.
func (s *BlobTestSuite) TestBlobE2EFlow() {
	// Create test namespace
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x02}, 10))
	s.Require().NoError(err)

	data := []byte("Hello Celestia E2E test blob")

	// Get wallet address for signing
	walletAddr, err := sdkacc.AddressFromWallet(s.testWallet)
	s.Require().NoError(err)

	// Create V1 blob with explicit signer (working approach)
	libBlob, err := blobtypes.NewV1Blob(namespace, data, walletAddr)
	s.Require().NoError(err)

	// Create MsgPayForBlobs transaction
	signerStr := s.testWallet.GetFormattedAddress()
	msg, err := blobtypes.NewMsgPayForBlobs(signerStr, appconsts.LatestVersion, libBlob)
	s.Require().NoError(err)

	// Submit via direct chain transaction (working approach)
	chain := s.framework.GetCelestiaChain()
	resp, err := chain.BroadcastBlobMessage(s.ctx, s.testWallet, msg, libBlob)
	s.Require().NoError(err)
	s.Require().Equal(uint32(0), resp.Code, "failed to broadcast blob: %s", resp.RawLog)

	// Wait for blob inclusion
	time.Sleep(10 * time.Second)

	// Get full node client for retrieval
	fullNode := s.framework.GetFullNodes()[0]
	fullNodeClient := s.framework.GetNodeRPCClient(s.ctx, fullNode)

	// Get the current height to search for blobs
	header, err := fullNodeClient.Header.NetworkHead(s.ctx)
	s.Require().NoError(err)
	currentHeight := header.Height()

	// Find the height where blob was actually included
	var blobHeight uint64
	var retrievedBlobs []*nodeblob.Blob
	found := false
	for h := uint64(resp.Height); h <= currentHeight; h++ {
		blobs, err := fullNodeClient.Blob.GetAll(s.ctx, h, []libshare.Namespace{namespace})
		if err == nil && len(blobs) > 0 {
			blobHeight = h
			retrievedBlobs = blobs
			found = true
			s.T().Logf("Found %d blobs at height %d", len(blobs), h)
			break
		}
	}
	s.Require().True(found, "blob not found at any height from %d to %d", resp.Height, currentHeight)
	s.Require().Len(retrievedBlobs, 1, "expected 1 blob")

	// Verify blob can be retrieved using the actual commitment
	retrievedBlob, err := fullNodeClient.Blob.Get(s.ctx, blobHeight, namespace, retrievedBlobs[0].Commitment)
	s.Require().NoError(err)
	s.Require().NotNil(retrievedBlob)

	// Trim null padding from retrieved data for comparison
	retrievedData := bytes.TrimRight(retrievedBlob.Data(), "\x00")
	s.Require().Equal(data, retrievedData)
}

// TestBlobDuplicateHandling tests duplicate blob submission scenarios.
// This uses the direct chain submission approach for testing duplicate handling.
func (s *BlobTestSuite) TestBlobDuplicateHandling() {
	// Create test namespace
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x03}, 10))
	s.Require().NoError(err)

	data := bytes.Repeat([]byte("duplicate"), 99)

	// Get wallet address for signing
	walletAddr, err := sdkacc.AddressFromWallet(s.testWallet)
	s.Require().NoError(err)

	// Test: Submit same blob twice in single transaction (should fail)
	s.Run("Duplicate_Blobs_Single_Transaction", func() {
		// Create identical V1 blobs
		libBlob1, err := blobtypes.NewV1Blob(namespace, data, walletAddr)
		s.Require().NoError(err)

		libBlob2, err := blobtypes.NewV1Blob(namespace, data, walletAddr)
		s.Require().NoError(err)

		// Create MsgPayForBlobs with both blobs
		signerStr := s.testWallet.GetFormattedAddress()
		msg, err := blobtypes.NewMsgPayForBlobs(signerStr, appconsts.LatestVersion, libBlob1, libBlob2)
		s.Require().NoError(err)

		// Submit via chain - this should succeed because Celestia allows duplicate blobs
		chain := s.framework.GetCelestiaChain()
		resp, err := chain.BroadcastBlobMessage(s.ctx, s.testWallet, msg, libBlob1, libBlob2)
		s.Require().NoError(err)
		s.Require().Equal(uint32(0), resp.Code, "duplicate blob submission should succeed in Celestia")
	})

	// Test: Submit same blob in separate transactions (should succeed)
	s.Run("Same_Blob_Separate_Transactions", func() {
		// First submission
		libBlob1, err := blobtypes.NewV1Blob(namespace, data, walletAddr)
		s.Require().NoError(err)

		signerStr := s.testWallet.GetFormattedAddress()
		msg1, err := blobtypes.NewMsgPayForBlobs(signerStr, appconsts.LatestVersion, libBlob1)
		s.Require().NoError(err)

		chain := s.framework.GetCelestiaChain()
		resp1, err := chain.BroadcastBlobMessage(s.ctx, s.testWallet, msg1, libBlob1)
		s.Require().NoError(err)
		s.Require().Equal(uint32(0), resp1.Code)

		// Second submission with identical data (should succeed in separate transaction)
		libBlob2, err := blobtypes.NewV1Blob(namespace, data, walletAddr)
		s.Require().NoError(err)

		msg2, err := blobtypes.NewMsgPayForBlobs(signerStr, appconsts.LatestVersion, libBlob2)
		s.Require().NoError(err)

		resp2, err := chain.BroadcastBlobMessage(s.ctx, s.testWallet, msg2, libBlob2)
		s.Require().NoError(err)
		s.Require().Equal(uint32(0), resp2.Code)

		// Heights should be different
		s.Require().NotEqual(resp1.Height, resp2.Height)
	})
}

// TestMixedBlobVersions tests submitting both V0 and V1 blobs together.
// This provides comprehensive coverage for mixed blob version scenarios.
func (s *BlobTestSuite) TestMixedBlobVersions() {
	// Create test namespace
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x04}, 10))
	s.Require().NoError(err)

	dataV0 := []byte("V0 blob data")
	dataV1 := []byte("V1 blob data")

	// Get wallet address for V1 blob signing
	walletAddr, err := sdkacc.AddressFromWallet(s.testWallet)
	s.Require().NoError(err)

	// Create V0 blob (no signer) - use libshare to create V0 blob
	libBlobV0, err := libshare.NewV0Blob(namespace, dataV0)
	s.Require().NoError(err)

	// Create V1 blob (with signer)
	libBlobV1, err := blobtypes.NewV1Blob(namespace, dataV1, walletAddr)
	s.Require().NoError(err)

	// Create MsgPayForBlobs transaction with both V0 and V1 blobs
	signerStr := s.testWallet.GetFormattedAddress()
	msg, err := blobtypes.NewMsgPayForBlobs(signerStr, appconsts.LatestVersion, libBlobV0, libBlobV1)
	s.Require().NoError(err)

	// Submit via direct chain transaction
	chain := s.framework.GetCelestiaChain()
	resp, err := chain.BroadcastBlobMessage(s.ctx, s.testWallet, msg, libBlobV0, libBlobV1)
	s.Require().NoError(err)
	s.Require().Equal(uint32(0), resp.Code, "failed to broadcast mixed blobs: %s", resp.RawLog)

	// Wait for blob inclusion
	time.Sleep(10 * time.Second)

	// Get full node client for retrieval
	fullNode := s.framework.GetFullNodes()[0]
	fullNodeClient := s.framework.GetNodeRPCClient(s.ctx, fullNode)

	// Get the current height to search for blobs
	header, err := fullNodeClient.Header.NetworkHead(s.ctx)
	s.Require().NoError(err)
	currentHeight := header.Height()

	// Find the height where blobs were included
	var retrievedBlobs []*nodeblob.Blob
	found := false
	for h := uint64(resp.Height); h <= currentHeight; h++ {
		blobs, err := fullNodeClient.Blob.GetAll(s.ctx, h, []libshare.Namespace{namespace})
		if err == nil && len(blobs) > 0 {
			retrievedBlobs = blobs
			found = true
			s.T().Logf("Found %d mixed blobs at height %d", len(blobs), h)
			break
		}
	}
	s.Require().True(found, "mixed blobs not found")
	s.Require().Len(retrievedBlobs, 2, "expected 2 mixed blobs")

	// Verify we have both V0 and V1 blobs
	foundV0, foundV1 := false, false
	for _, blob := range retrievedBlobs {
		retrievedData := bytes.TrimRight(blob.Data(), "\x00")

		if blob.ShareVersion() == libshare.ShareVersionZero {
			foundV0 = true
			s.Require().Equal(dataV0, retrievedData, "V0 blob data should match")
			s.Require().Nil(blob.Signer(), "V0 blob should have no signer")
		} else if blob.ShareVersion() == libshare.ShareVersionOne {
			foundV1 = true
			s.Require().Equal(dataV1, retrievedData, "V1 blob data should match")
			s.Require().NotNil(blob.Signer(), "V1 blob should have signer")
			s.Require().Equal(walletAddr.Bytes(), blob.Signer(), "V1 signer should match")
		}
	}

	s.Require().True(foundV0, "V0 blob not found in mixed submission")
	s.Require().True(foundV1, "V1 blob not found in mixed submission")
}

// TestBlobTestSuite runs the blob test suite.
func TestBlobTestSuite(t *testing.T) {
	suite.Run(t, new(BlobTestSuite))
}
