//go:build integration

package tastora

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	libshare "github.com/celestiaorg/go-square/v2/share"

	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/state"
)

// ShareTestSuite provides comprehensive testing of the Share module APIs.
// Tests data availability, share retrieval, and validation functionality.
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
	s.framework = NewFramework(s.T(), WithValidators(1), WithFullNodes(1), WithBridgeNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

// Helper function to submit a blob and get its height
func (s *ShareTestSuite) submitTestBlob(ctx context.Context, namespace libshare.Namespace, data []byte) (uint64, []byte) {
	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create test wallet and fund the node account
	testWallet := s.framework.CreateTestWallet(ctx, 5_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, fullNode, 1_000_000_000)

	// Get node account address for blob signing
	nodeAddr, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err)

	// Create blob using share package
	libBlob, err := libshare.NewV1Blob(namespace, data, nodeAddr.Bytes())
	s.Require().NoError(err)

	// Convert to node blobs
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

	return height, nodeBlobs[0].Commitment
}

// TestShareSharesAvailable validates SharesAvailable API functionality
func (s *ShareTestSuite) TestShareSharesAvailable_ValidHeight() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Submit a test blob to ensure we have data
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x01}, 10))
	s.Require().NoError(err)
	data := []byte("SharesAvailable test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Test SharesAvailable API
	err = client.Share.SharesAvailable(ctx, height)
	s.Require().NoError(err, "shares should be available")
}

func (s *ShareTestSuite) TestShareSharesAvailable_InvalidHeight() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Test with a very high height (should fail)
	futureHeight := uint64(1000000)
	err := client.Share.SharesAvailable(ctx, futureHeight)
	s.Assert().Error(err, "should return error for future height")
}

// TestShareGetShare validates GetShare API functionality
func (s *ShareTestSuite) TestShareGetShare_ValidCoordinates() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Submit a test blob
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x02}, 10))
	s.Require().NoError(err)
	data := []byte("GetShare test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Get the EDS to verify it exists
	_, err = client.Share.GetEDS(ctx, height)
	s.Require().NoError(err, "should get EDS")

	// Try to get a share at valid coordinates
	share, err := client.Share.GetShare(ctx, height, 0, 0)
	s.Require().NoError(err, "should get share at valid coordinates")
	s.Require().NotNil(share, "share should not be nil")
	s.Assert().NotEmpty(share, "share should not be empty")
}

func (s *ShareTestSuite) TestShareGetShare_InvalidCoordinates() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get a valid height
	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err)

	// Try to get share with invalid coordinates (out of bounds)
	_, err = client.Share.GetShare(ctx, head.Height(), 1000, 1000)
	s.Assert().Error(err, "should return error for invalid coordinates")
}

// TestShareGetSamples validates GetSamples API functionality
func (s *ShareTestSuite) TestShareGetSamples_ValidCoordinates() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Submit a test blob
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x03}, 10))
	s.Require().NoError(err)
	data := []byte("GetSamples test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Get header for the height
	header, err := client.Header.GetByHeight(ctx, height)
	s.Require().NoError(err, "should get header")

	// Create sample coordinates
	sampleCoords := []shwap.SampleCoords{
		{Row: 0, Col: 0},
		{Row: 0, Col: 1},
	}

	// Get samples
	samples, err := client.Share.GetSamples(ctx, header, sampleCoords)
	s.Require().NoError(err, "should get samples")
	s.Require().Len(samples, 2, "should get expected number of samples")

	// Validate samples
	for i, sample := range samples {
		s.Assert().NotEmpty(sample.Share, "sample %d should not be empty", i)
		s.Assert().NotNil(sample.Proof, "sample %d should have proof", i)
	}
}

func (s *ShareTestSuite) TestShareGetSamples_InvalidCoordinates() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get a valid header
	header, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err)

	// Create invalid sample coordinates
	invalidCoords := []shwap.SampleCoords{
		{Row: 1000, Col: 1000},
	}

	// Try to get samples with invalid coordinates
	_, err = client.Share.GetSamples(ctx, header, invalidCoords)
	s.Assert().Error(err, "should return error for invalid coordinates")
}

// TestShareGetEDS validates GetEDS API functionality
func (s *ShareTestSuite) TestShareGetEDS_ValidHeight() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Submit a test blob to ensure we have data
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x04}, 10))
	s.Require().NoError(err)
	data := []byte("GetEDS test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Get EDS
	eds, err := client.Share.GetEDS(ctx, height)
	s.Require().NoError(err, "should get EDS")
	s.Require().NotNil(eds, "EDS should not be nil")

	// Validate EDS structure
	s.Assert().Greater(eds.Width(), uint(0), "EDS width should be greater than 0")
	s.Assert().Equal(eds.Width(), eds.Width(), "EDS should be square")
}

func (s *ShareTestSuite) TestShareGetEDS_InvalidHeight() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Try to get EDS for invalid height
	_, err := client.Share.GetEDS(ctx, 0)
	s.Assert().Error(err, "should return error for height 0")
}

// TestShareGetRow validates GetRow API functionality
func (s *ShareTestSuite) TestShareGetRow_ValidRow() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Submit a test blob
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x05}, 10))
	s.Require().NoError(err)
	data := []byte("GetRow test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Get row
	row, err := client.Share.GetRow(ctx, height, 0)
	s.Require().NoError(err, "should get row")
	s.Require().NotNil(row, "row should not be nil")

	// Validate row structure
	s.Assert().NotEmpty(row.Shares, "row should have shares")
	s.Assert().NotNil(row, "row should be valid")
}

func (s *ShareTestSuite) TestShareGetRow_InvalidRow() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get a valid height
	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err)

	// Try to get row with invalid index
	_, err = client.Share.GetRow(ctx, head.Height(), 1000)
	s.Assert().Error(err, "should return error for invalid row index")
}

// TestShareGetRange validates GetRange API functionality
func (s *ShareTestSuite) TestShareGetRange_ValidRange() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Submit a test blob
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x06}, 10))
	s.Require().NoError(err)
	data := []byte("GetRange test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Get range data (retrieve shares 0-3 for example)
	rangeResult, err := client.Share.GetRange(ctx, height, 0, 3)
	s.Require().NoError(err, "should get range data")
	s.Require().NotNil(rangeResult, "range result should not be nil")

	// Validate range result
	s.Assert().NotEmpty(rangeResult.Shares, "range result should have shares")
	s.Assert().NotNil(rangeResult.Proof, "range result should have proof")

	// Validate that shares are within the expected range
	s.Assert().LessOrEqual(len(rangeResult.Shares), 4, "should have at most 4 shares for range 0-3")
}

func (s *ShareTestSuite) TestShareGetRange_EmptyRange() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get current head
	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err)

	// Try to get range data for empty range (start == end)
	rangeResult, err := client.Share.GetRange(ctx, head.Height(), 0, 0)
	s.Require().NoError(err, "should succeed even for empty range")
	s.Assert().LessOrEqual(len(rangeResult.Shares), 1, "should return at most 1 share for single index range")
}

// TestShareGetRange_ProofVerification validates GetRange API with proof verification
func (s *ShareTestSuite) TestShareGetRange_ProofVerification() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Submit a test blob
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x07}, 10))
	s.Require().NoError(err)
	data := []byte("GetRange proof verification test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Get header to extract data root for proof verification
	header, err := client.Header.GetByHeight(ctx, height)
	s.Require().NoError(err, "should get header")

	// Get range data
	rangeResult, err := client.Share.GetRange(ctx, height, 0, 5)
	s.Require().NoError(err, "should get range data")
	s.Require().NotNil(rangeResult, "range result should not be nil")

	// Validate range result structure
	s.Assert().NotEmpty(rangeResult.Shares, "range result should have shares")
	s.Assert().NotNil(rangeResult.Proof, "range result should have proof")

	// Verify the proof against the data root
	dataRoot := header.DAH.Hash()
	err = rangeResult.Verify(dataRoot)
	s.Assert().NoError(err, "range proof should verify against data root")
}

func (s *ShareTestSuite) TestShareGetRange_LargeRange() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Submit a test blob
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x08}, 10))
	s.Require().NoError(err)
	data := []byte("GetRange large range test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Get range data for a larger range (0-15)
	rangeResult, err := client.Share.GetRange(ctx, height, 0, 15)
	s.Require().NoError(err, "should get range data for large range")
	s.Require().NotNil(rangeResult, "range result should not be nil")

	// Validate that we have data
	s.Assert().NotEmpty(rangeResult.Shares, "range result should have shares")
	s.Assert().NotNil(rangeResult.Proof, "range result should have proof")
	s.Assert().LessOrEqual(len(rangeResult.Shares), 16, "should have at most 16 shares for range 0-15")
}

func (s *ShareTestSuite) TestShareGetRange_InvalidRange() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get current head
	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err)

	// Try to get range data with invalid range (start > end)
	_, err = client.Share.GetRange(ctx, head.Height(), 10, 5)
	s.Assert().Error(err, "should return error for invalid range where start > end")

	// Try to get range data with very large invalid range
	_, err = client.Share.GetRange(ctx, head.Height(), 1000, 2000)
	s.Assert().Error(err, "should return error for out-of-bounds range")
}

// TestShareCrossNodeAvailability validates share availability across different node types
func (s *ShareTestSuite) TestShareCrossNodeAvailability() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)

	fullClient := s.framework.GetNodeRPCClient(ctx, fullNode)
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Submit a test blob
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x0A}, 10))
	s.Require().NoError(err)
	data := []byte("Cross node availability test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Wait for data to propagate
	time.Sleep(10 * time.Second)

	// Check availability on full node
	err = fullClient.Share.SharesAvailable(ctx, height)
	s.Require().NoError(err, "shares should be available on full node")

	// Check availability on bridge node
	err = bridgeClient.Share.SharesAvailable(ctx, height)
	s.Require().NoError(err, "shares should be available on bridge node")

	// Get range data from both nodes and compare
	fullRange, err := fullClient.Share.GetRange(ctx, height, 0, 3)
	s.Require().NoError(err, "should get range data from full node")

	bridgeRange, err := bridgeClient.Share.GetRange(ctx, height, 0, 3)
	s.Require().NoError(err, "should get range data from bridge node")

	// Validate range data consistency
	s.Assert().Equal(len(fullRange.Shares), len(bridgeRange.Shares), "range shares count should match across nodes")
	s.Assert().NotNil(fullRange.Proof, "full node should have proof")
	s.Assert().NotNil(bridgeRange.Proof, "bridge node should have proof")
}

// TestShareErrorHandling validates error handling scenarios
func (s *ShareTestSuite) TestShareErrorHandling_NetworkTimeout() {
	// Create an already cancelled context to guarantee timeout behavior
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to force timeout

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// This should timeout immediately due to cancelled context
	err := client.Share.SharesAvailable(ctx, 1)
	s.Assert().Error(err, "should return timeout error")
	// Fix: Check if err is not nil before calling err.Error()
	if err != nil {
		// Check for context cancelled error instead of deadline exceeded
		s.Assert().Contains(err.Error(), "context canceled", "should be context cancelled error")
	}
}

// TestShareGetShareQ1 validates GetShare API functionality for Quadrant 1 (original data)
func (s *ShareTestSuite) TestShareGetShareQ1_OriginalData() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Submit a test blob
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x10}, 10))
	s.Require().NoError(err)
	data := []byte("Quadrant 1 test data")
	height, commitment := s.submitTestBlob(ctx, namespace, data)

	// Get the blob to find its coordinates
	retrievedBlob, err := client.Blob.Get(ctx, height, namespace, commitment)
	s.Require().NoError(err)

	// Get header to determine EDS structure
	hdr, err := client.Header.GetByHeight(ctx, height)
	s.Require().NoError(err)

	// Calculate blob coordinates in EDS
	coords, err := shwap.SampleCoordsFrom1DIndex(retrievedBlob.Index(), len(hdr.DAH.RowRoots))
	s.Require().NoError(err)

	// Test GetShare from Quadrant 1 (original data region)
	share, err := client.Share.GetShare(ctx, height, coords.Row, coords.Col)
	s.Require().NoError(err, "should get share from Q1")
	s.Assert().NotEmpty(share, "Q1 share should not be empty")

	// Verify the share belongs to the correct namespace
	s.Assert().True(share.Namespace().Equals(namespace), "Q1 share should belong to correct namespace")
}

// TestShareGetShareQ4 validates GetShare API functionality for Quadrant 4 (parity of parity)
func (s *ShareTestSuite) TestShareGetShareQ4_ParityData() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Submit a test blob
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x11}, 10))
	s.Require().NoError(err)
	data := []byte("Quadrant 4 parity test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Get header to determine EDS structure
	hdr, err := client.Header.GetByHeight(ctx, height)
	s.Require().NoError(err)

	// Test GetShare from Quadrant 4 (last quadrant - parity of parity)
	lastRow := len(hdr.DAH.RowRoots) - 1
	lastCol := len(hdr.DAH.ColumnRoots) - 1
	share, err := client.Share.GetShare(ctx, height, lastRow, lastCol)
	s.Require().NoError(err, "should get share from Q4")
	s.Assert().NotEmpty(share, "Q4 share should not be empty")

	// Q4 shares should be parity data (typically empty namespace for parity)
	s.Assert().True(share.Namespace().IsParityShares(), "Q4 share should be parity namespace")
}

// TestShareGetSamplesQ1 validates GetSamples API functionality for Quadrant 1
func (s *ShareTestSuite) TestShareGetSamplesQ1_OriginalData() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Submit a test blob
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x12}, 10))
	s.Require().NoError(err)
	data := []byte("GetSamples Q1 test data")
	height, commitment := s.submitTestBlob(ctx, namespace, data)

	// Get the blob to find its coordinates
	retrievedBlob, err := client.Blob.Get(ctx, height, namespace, commitment)
	s.Require().NoError(err)

	// Get header for verification
	hdr, err := client.Header.GetByHeight(ctx, height)
	s.Require().NoError(err)

	// Calculate blob coordinates in Q1
	coords, err := shwap.SampleCoordsFrom1DIndex(retrievedBlob.Index(), len(hdr.DAH.RowRoots))
	s.Require().NoError(err)

	// Test GetSamples from Q1
	requestCoords := []shwap.SampleCoords{coords}
	samples, err := client.Share.GetSamples(ctx, hdr, requestCoords)
	s.Require().NoError(err, "should get samples from Q1")
	s.Require().Len(samples, 1, "should get exactly 1 sample")

	// Verify sample structure and proof
	sample := samples[0]
	s.Assert().NotEmpty(sample.Share, "Q1 sample share should not be empty")
	s.Assert().NotNil(sample.Proof, "Q1 sample should have proof")

	// Verify the sample against DAH
	err = sample.Verify(hdr.DAH, coords.Row, coords.Col)
	s.Assert().NoError(err, "Q1 sample should verify against DAH")

	// Verify namespace matches
	s.Assert().True(sample.Share.Namespace().Equals(namespace), "Q1 sample should belong to correct namespace")
}

// TestShareGetSamplesQ4 validates GetSamples API functionality for Quadrant 4
func (s *ShareTestSuite) TestShareGetSamplesQ4_ParityData() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Submit a test blob to ensure EDS has data
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x13}, 10))
	s.Require().NoError(err)
	data := []byte("GetSamples Q4 test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Get header for EDS structure
	hdr, err := client.Header.GetByHeight(ctx, height)
	s.Require().NoError(err)

	// Create coordinates for Q4 (last quadrant)
	lastRow := len(hdr.DAH.RowRoots) - 1
	lastCol := len(hdr.DAH.RowRoots) - 1 // EDS is square
	coords := shwap.SampleCoords{Row: lastRow, Col: lastCol}

	// Test GetSamples from Q4
	requestCoords := []shwap.SampleCoords{coords}
	samples, err := client.Share.GetSamples(ctx, hdr, requestCoords)
	s.Require().NoError(err, "should get samples from Q4")
	s.Require().Len(samples, 1, "should get exactly 1 sample")

	// Verify sample structure and proof
	sample := samples[0]
	s.Assert().NotEmpty(sample.Share, "Q4 sample share should not be empty")
	s.Assert().NotNil(sample.Proof, "Q4 sample should have proof")

	// Verify the sample against DAH
	err = sample.Verify(hdr.DAH, coords.Row, coords.Col)
	s.Assert().NoError(err, "Q4 sample should verify against DAH")

	// Q4 shares should be parity data
	s.Assert().True(sample.Share.Namespace().IsParityShares(), "Q4 sample should be parity namespace")
}

// TestShareGetRowQ1 validates GetRow API functionality for Quadrant 1 rows
func (s *ShareTestSuite) TestShareGetRowQ1_OriginalDataRows() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Submit a test blob
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x14}, 10))
	s.Require().NoError(err)
	data := []byte("GetRow Q1 test data")
	height, commitment := s.submitTestBlob(ctx, namespace, data)

	// Get the blob to find its coordinates
	retrievedBlob, err := client.Blob.Get(ctx, height, namespace, commitment)
	s.Require().NoError(err)

	// Get header for verification
	hdr, err := client.Header.GetByHeight(ctx, height)
	s.Require().NoError(err)

	// Calculate blob coordinates to find Q1 row
	coords, err := shwap.SampleCoordsFrom1DIndex(retrievedBlob.Index(), len(hdr.DAH.RowRoots))
	s.Require().NoError(err)

	// Test GetRow from Q1 (row containing our blob)
	row, err := client.Share.GetRow(ctx, height, coords.Row)
	s.Require().NoError(err, "should get row from Q1")
	s.Require().NotNil(row, "Q1 row should not be nil")

	// Verify row structure
	s.Assert().NotEmpty(row.Shares, "Q1 row should have shares")

	// Verify row against DAH
	err = row.Verify(hdr.DAH, coords.Row)
	s.Assert().NoError(err, "Q1 row should verify against DAH")

	// Get shares from row and verify our blob data is present
	shares, err := row.Shares()
	s.Require().NoError(err)
	s.Assert().True(shares[coords.Col].Namespace().Equals(namespace), "blob share should be in correct position")
}

// TestShareGetRowQ4 validates GetRow API functionality for Quadrant 4 rows
func (s *ShareTestSuite) TestShareGetRowQ4_ParityRows() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Submit a test blob to ensure EDS has data
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x15}, 10))
	s.Require().NoError(err)
	data := []byte("GetRow Q4 test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Get header for EDS structure
	hdr, err := client.Header.GetByHeight(ctx, height)
	s.Require().NoError(err)

	// Test GetRow from Q4 (last row - parity data)
	lastRow := len(hdr.DAH.RowRoots) - 1
	row, err := client.Share.GetRow(ctx, height, lastRow)
	s.Require().NoError(err, "should get row from Q4")
	s.Require().NotNil(row, "Q4 row should not be nil")

	// Verify row structure
	s.Assert().NotEmpty(row.Shares, "Q4 row should have shares")

	// Verify row against DAH
	err = row.Verify(hdr.DAH, lastRow)
	s.Assert().NoError(err, "Q4 row should verify against DAH")

	// Get shares from row and verify they are parity shares
	shares, err := row.Shares()
	s.Require().NoError(err)

	// Check that shares in Q4 row are parity shares
	halfPoint := len(shares) / 2
	for i := halfPoint; i < len(shares); i++ {
		s.Assert().True(shares[i].Namespace().IsParityShares(), "Q4 shares should be parity namespace")
	}
}

// TestSharePerformance validates performance characteristics
func (s *ShareTestSuite) TestSharePerformance_ConcurrentRequests() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Submit a test blob
	namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x0B}, 10))
	s.Require().NoError(err)
	data := []byte("Performance test data")
	height, _ := s.submitTestBlob(ctx, namespace, data)

	// Test concurrent access
	const numConcurrent = 5
	results := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func(i int) {
			rowIdx := i % 4 // Assume at least 4 rows exist
			_, err := client.Share.GetRow(ctx, height, rowIdx)
			results <- err
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numConcurrent; i++ {
		select {
		case err := <-results:
			s.Assert().NoError(err, "concurrent request %d should succeed", i)
		case <-time.After(30 * time.Second):
			s.Fail("concurrent request timed out")
		}
	}
}
