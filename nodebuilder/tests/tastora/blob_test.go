//go:build integration

package tastora

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/celestia-app/v4/pkg/appconsts"
	blobtypes "github.com/celestiaorg/celestia-app/v4/x/blob/types"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/tastora/framework/testutil/sdkacc"

	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/state"
)

// BlobTestSuite provides comprehensive testing of the Blob module APIs.
// Tests blob submission, retrieval, and validation functionality.
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
	s.framework = NewFramework(s.T(), WithValidators(1), WithFullNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *BlobTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.cleanup()
	}
}

// TestBlobSubmit_SingleBlob tests blob submission API with a single blob
func (s *BlobTestSuite) TestBlobSubmit_SingleBlob() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create test wallet and fund the node account
	testWallet := s.framework.CreateTestWallet(ctx, 5_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, fullNode, 3_000_000_000)

	// Create test blob
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x01}, 10))
	s.Require().NoError(err)

	data := []byte("Hello Celestia single blob test")
	nodeAddr, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err)

	libBlob, err := libshare.NewV1Blob(namespace, data, nodeAddr.Bytes())
	s.Require().NoError(err)

	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlob)
	s.Require().NoError(err)

	// Submit blob via Submit API
	txConfig := state.NewTxConfig(state.WithGas(200_000), state.WithGasPrice(5000))
	height, err := client.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err)
	s.Require().NotZero(height)

	// Wait for inclusion
	_, err = client.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err)

	// Verify blob can be retrieved
	retrievedBlob, err := client.Blob.Get(ctx, height, namespace, nodeBlobs[0].Commitment)
	s.Require().NoError(err)
	s.Require().NotNil(retrievedBlob)

	retrievedData := bytes.TrimRight(retrievedBlob.Data(), "\x00")
	s.Assert().Equal(data, retrievedData)
	s.Assert().True(retrievedBlob.Namespace().Equals(namespace))
}

// TestBlobSubmit_MultipleBlobs tests blob submission API with multiple blobs
func (s *BlobTestSuite) TestBlobSubmit_MultipleBlobs() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create test wallet and fund the node account
	testWallet := s.framework.CreateTestWallet(ctx, 5_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, fullNode, 3_000_000_000)

	// Create test namespace and blobs
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x02}, 10))
	s.Require().NoError(err)

	nodeAddr, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err)

	data1 := []byte("Multiple blob test data 1")
	data2 := []byte("Multiple blob test data 2")

	libBlob1, err := libshare.NewV1Blob(namespace, data1, nodeAddr.Bytes())
	s.Require().NoError(err)
	libBlob2, err := libshare.NewV1Blob(namespace, data2, nodeAddr.Bytes())
	s.Require().NoError(err)

	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlob1, libBlob2)
	s.Require().NoError(err)

	// Submit multiple blobs
	txConfig := state.NewTxConfig(state.WithGas(400_000), state.WithGasPrice(5000))
	height, err := client.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err)
	s.Require().NotZero(height)

	// Wait for inclusion
	_, err = client.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err)

	// Verify all blobs can be retrieved
	retrievedBlobs, err := client.Blob.GetAll(ctx, height, []libshare.Namespace{namespace})
	s.Require().NoError(err)
	s.Require().Len(retrievedBlobs, 2)

	// Verify individual blob retrieval
	for i, nodeBlob := range nodeBlobs {
		retrievedBlob, err := client.Blob.Get(ctx, height, namespace, nodeBlob.Commitment)
		s.Require().NoError(err)
		s.Require().NotNil(retrievedBlob)

		retrievedData := bytes.TrimRight(retrievedBlob.Data(), "\x00")
		if i == 0 {
			s.Assert().Equal(data1, retrievedData)
		} else {
			s.Assert().Equal(data2, retrievedData)
		}
	}
}

// TestBlobGet_ExistingBlob tests blob retrieval API for existing blobs
func (s *BlobTestSuite) TestBlobGet_ExistingBlob() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use chain submission for reliable blob placement
	testWallet := s.framework.CreateTestWallet(ctx, 5_000_000_000)
	walletAddr, err := sdkacc.AddressFromWallet(testWallet)
	s.Require().NoError(err)

	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x03}, 10))
	s.Require().NoError(err)

	data := []byte("Test blob for Get API")
	libBlob, err := blobtypes.NewV1Blob(namespace, data, walletAddr)
	s.Require().NoError(err)

	// Submit via chain
	signerStr := testWallet.GetFormattedAddress()
	msg, err := blobtypes.NewMsgPayForBlobs(signerStr, appconsts.LatestVersion, libBlob)
	s.Require().NoError(err)

	chain := s.framework.GetCelestiaChain()
	resp, err := chain.BroadcastBlobMessage(ctx, testWallet, msg, libBlob)
	s.Require().NoError(err)
	s.Require().Equal(uint32(0), resp.Code)

	// Wait and find blob
	time.Sleep(5 * time.Second)
	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	header, err := client.Header.NetworkHead(ctx)
	s.Require().NoError(err)

	var blobHeight uint64
	var commitment []byte
	found := false

	for h := uint64(resp.Height); h <= header.Height(); h++ {
		blobs, err := client.Blob.GetAll(ctx, h, []libshare.Namespace{namespace})
		if err == nil && len(blobs) > 0 {
			blobHeight = h
			commitment = blobs[0].Commitment
			found = true
			break
		}
	}
	s.Require().True(found, "blob should be found")

	// Test Get API
	retrievedBlob, err := client.Blob.Get(ctx, blobHeight, namespace, commitment)
	s.Require().NoError(err)
	s.Require().NotNil(retrievedBlob)

	retrievedData := bytes.TrimRight(retrievedBlob.Data(), "\x00")
	s.Assert().Equal(data, retrievedData)
	s.Assert().True(retrievedBlob.Namespace().Equals(namespace))
	s.Assert().Equal(libshare.ShareVersionOne, retrievedBlob.ShareVersion())
}

// TestBlobGet_NonExistentBlob tests blob retrieval API error handling
func (s *BlobTestSuite) TestBlobGet_NonExistentBlob() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x04}, 10))
	s.Require().NoError(err)

	// Try to get non-existent blob
	nonExistentCommitment := bytes.Repeat([]byte{0xFF}, 32)
	_, err = client.Blob.Get(ctx, 1, namespace, nonExistentCommitment)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "blob: not found")
}

// TestBlobGetAll_ValidNamespace tests GetAll API with valid namespace
func (s *BlobTestSuite) TestBlobGetAll_ValidNamespace() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create test wallet and fund the node account
	testWallet := s.framework.CreateTestWallet(ctx, 5_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, fullNode, 3_000_000_000)

	// Create test namespace
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x05}, 10))
	s.Require().NoError(err)

	nodeAddr, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err)

	// Submit multiple blobs to same namespace
	data1 := []byte("GetAll test data 1")
	data2 := []byte("GetAll test data 2")

	libBlob1, err := libshare.NewV1Blob(namespace, data1, nodeAddr.Bytes())
	s.Require().NoError(err)
	libBlob2, err := libshare.NewV1Blob(namespace, data2, nodeAddr.Bytes())
	s.Require().NoError(err)

	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlob1, libBlob2)
	s.Require().NoError(err)

	txConfig := state.NewTxConfig(state.WithGas(400_000), state.WithGasPrice(5000))
	height, err := client.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err)

	// Wait for inclusion
	_, err = client.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err)

	// Test GetAll API
	retrievedBlobs, err := client.Blob.GetAll(ctx, height, []libshare.Namespace{namespace})
	s.Require().NoError(err)
	s.Require().Len(retrievedBlobs, 2)

	// Verify all blobs belong to correct namespace
	for _, blob := range retrievedBlobs {
		s.Assert().True(blob.Namespace().Equals(namespace))
	}
}

// TestBlobGetProof_ValidBlob tests GetProof API functionality
func (s *BlobTestSuite) TestBlobGetProof_ValidBlob() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create test wallet and fund the node account
	testWallet := s.framework.CreateTestWallet(ctx, 5_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, fullNode, 3_000_000_000)

	// Create and submit test blob
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x06}, 10))
	s.Require().NoError(err)

	data := []byte("Proof test data")
	nodeAddr, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err)

	libBlob, err := libshare.NewV1Blob(namespace, data, nodeAddr.Bytes())
	s.Require().NoError(err)

	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlob)
	s.Require().NoError(err)

	txConfig := state.NewTxConfig(state.WithGas(200_000), state.WithGasPrice(5000))
	height, err := client.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err)

	// Wait for inclusion
	_, err = client.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err)

	// Test GetProof API
	proof, err := client.Blob.GetProof(ctx, height, namespace, nodeBlobs[0].Commitment)
	s.Require().NoError(err)
	s.Require().NotNil(proof)
	s.Require().NotEmpty(proof)

	// Test Included API with the proof
	included, err := client.Blob.Included(ctx, height, namespace, proof, nodeBlobs[0].Commitment)
	s.Require().NoError(err)
	s.Assert().True(included)
}

// TestBlobMixedVersions tests mixed V0 and V1 blob scenarios
func (s *BlobTestSuite) TestBlobMixedVersions() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use chain submission for mixed version testing
	testWallet := s.framework.CreateTestWallet(ctx, 5_000_000_000)
	walletAddr, err := sdkacc.AddressFromWallet(testWallet)
	s.Require().NoError(err)

	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x07}, 10))
	s.Require().NoError(err)

	dataV0 := []byte("V0 blob data")
	dataV1 := []byte("V1 blob data")

	// Create V0 blob (no signer)
	libBlobV0, err := libshare.NewV0Blob(namespace, dataV0)
	s.Require().NoError(err)

	// Create V1 blob (with signer)
	libBlobV1, err := blobtypes.NewV1Blob(namespace, dataV1, walletAddr)
	s.Require().NoError(err)

	// Submit mixed blobs via chain
	signerStr := testWallet.GetFormattedAddress()
	msg, err := blobtypes.NewMsgPayForBlobs(signerStr, appconsts.LatestVersion, libBlobV0, libBlobV1)
	s.Require().NoError(err)

	chain := s.framework.GetCelestiaChain()
	resp, err := chain.BroadcastBlobMessage(ctx, testWallet, msg, libBlobV0, libBlobV1)
	s.Require().NoError(err)
	s.Require().Equal(uint32(0), resp.Code)

	// Wait and retrieve blobs
	time.Sleep(5 * time.Second)
	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	header, err := client.Header.NetworkHead(ctx)
	s.Require().NoError(err)

	var retrievedBlobs []*nodeblob.Blob
	found := false

	for h := uint64(resp.Height); h <= header.Height(); h++ {
		blobs, err := client.Blob.GetAll(ctx, h, []libshare.Namespace{namespace})
		if err == nil && len(blobs) > 0 {
			retrievedBlobs = blobs
			found = true
			break
		}
	}
	s.Require().True(found, "mixed blobs should be found")
	s.Require().Len(retrievedBlobs, 2)

	// Verify mixed blob versions
	foundV0, foundV1 := false, false
	for _, blob := range retrievedBlobs {
		retrievedData := bytes.TrimRight(blob.Data(), "\x00")

		if blob.ShareVersion() == libshare.ShareVersionZero {
			foundV0 = true
			s.Assert().Equal(dataV0, retrievedData)
			s.Assert().Nil(blob.Signer())
		} else if blob.ShareVersion() == libshare.ShareVersionOne {
			foundV1 = true
			s.Assert().Equal(dataV1, retrievedData)
			s.Assert().NotNil(blob.Signer())
			s.Assert().Equal(walletAddr.Bytes(), blob.Signer())
		}
	}

	s.Assert().True(foundV0, "V0 blob should be found")
	s.Assert().True(foundV1, "V1 blob should be found")
}
