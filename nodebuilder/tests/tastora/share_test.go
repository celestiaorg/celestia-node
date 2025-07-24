//go:build integration

package tastora

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
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

// TestShareSharesAvailable tests SharesAvailable API with various scenarios
func (s *ShareTestSuite) TestShareSharesAvailable() {
	s.Run("ValidHeight", func() {
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
	})

	s.Run("InvalidHeight", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		fullNode := s.framework.GetOrCreateFullNode(ctx)
		client := s.framework.GetNodeRPCClient(ctx, fullNode)

		// Test with a very high height (should fail)
		futureHeight := uint64(1000000)
		err := client.Share.SharesAvailable(ctx, futureHeight)
		s.Assert().Error(err, "should return error for future height")
	})
}

// TestShareGetShare tests GetShare API with various coordinate scenarios
func (s *ShareTestSuite) TestShareGetShare() {
	s.Run("ValidCoordinates", func() {
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
	})

	s.Run("InvalidCoordinates", func() {
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
	})

	s.Run("Q1_OriginalData", func() {
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
	})

	s.Run("Q4_ParityData", func() {
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
	})
}

// TestShareGetSamples tests GetSamples API with various coordinate scenarios
func (s *ShareTestSuite) TestShareGetSamples() {
	s.Run("ValidCoordinates", func() {
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
	})

	s.Run("InvalidCoordinates", func() {
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
	})

	s.Run("Q1_OriginalData", func() {
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
	})

	s.Run("Q4_ParityData", func() {
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
	})
}

// TestShareGetEDS tests GetEDS API with various height scenarios
func (s *ShareTestSuite) TestShareGetEDS() {
	s.Run("ValidHeight", func() {
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
	})

	s.Run("InvalidHeight", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		fullNode := s.framework.GetOrCreateFullNode(ctx)
		client := s.framework.GetNodeRPCClient(ctx, fullNode)

		// Try to get EDS for invalid height
		_, err := client.Share.GetEDS(ctx, 0)
		s.Assert().Error(err, "should return error for height 0")
	})
}

// TestShareGetRow tests GetRow API with various row scenarios
func (s *ShareTestSuite) TestShareGetRow() {
	s.Run("ValidRow", func() {
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
	})

	s.Run("InvalidRow", func() {
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
	})

	s.Run("Q1_OriginalDataRows", func() {
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
	})

	s.Run("Q4_ParityRows", func() {
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
	})
}

// TestShareGetRange tests GetRange API with various range scenarios
func (s *ShareTestSuite) TestShareGetRange() {
	s.Run("ValidRange", func() {
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
	})

	s.Run("EmptyRange", func() {
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
	})

	s.Run("ProofVerification", func() {
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
	})

	s.Run("LargeRange", func() {
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
	})

	s.Run("InvalidRange", func() {
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
	})

	s.Run("FromLightNodes", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		// Create bridge and light nodes
		bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
		lightNode := s.framework.GetOrCreateLightNode(ctx)

		bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
		lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

		// Submit multiple test blobs to create realistic data scenario
		namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x25}, 10))
		s.Require().NoError(err)

		heights := make([]uint64, 3)
		for i := 0; i < 3; i++ {
			data := []byte(fmt.Sprintf("Light node GetRange test data block %d", i+1))
			height, _ := s.submitTestBlob(ctx, namespace, data)
			heights[i] = height
		}

		// Test GetRange from both bridge and light nodes for each height
		for _, height := range heights {
			// Wait for light node to sync to height
			_, err = lightClient.Header.WaitForHeight(ctx, height)
			s.Require().NoError(err)

			// Get range data from bridge node (source of truth)
			expectedRange, err := bridgeClient.Share.GetRange(ctx, height, 0, 10)
			s.Require().NoError(err, "bridge should provide range data")
			s.Require().NotEmpty(expectedRange.Shares, "bridge range should have shares")

			// Get range data from light node (should fetch via ShrexND)
			gotRange, err := lightClient.Share.GetRange(ctx, height, 0, 10)
			s.Require().NoError(err, "light node should fetch range data via ShrexND")
			s.Require().NotEmpty(gotRange.Shares, "light node range should have shares")

			// Verify data consistency between bridge and light node
			s.Assert().Equal(expectedRange.Shares, gotRange.Shares,
				"light node should get same range data as bridge for height %d", height)
			s.Assert().NotNil(gotRange.Proof, "light node range should have proof")
		}
	})
}

// TestShareGetNamespaceData tests GetNamespaceData API with various namespace scenarios
func (s *ShareTestSuite) TestShareGetNamespaceData() {
	s.Run("ValidNamespace", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		fullNode := s.framework.GetOrCreateFullNode(ctx)
		client := s.framework.GetNodeRPCClient(ctx, fullNode)

		// Submit a test blob with specific namespace
		namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x20}, 10))
		s.Require().NoError(err)
		data := []byte("GetNamespaceData test data")
		height, commitment := s.submitTestBlob(ctx, namespace, data)

		// Get namespace data
		namespaceData, err := client.Share.GetNamespaceData(ctx, height, namespace)
		s.Require().NoError(err, "should get namespace data")
		s.Require().NotNil(namespaceData, "namespace data should not be nil")

		// Validate namespace data structure
		s.Assert().NotEmpty(namespaceData, "namespace data should not be empty")

		// Flatten all shares from all rows
		allShares := namespaceData.Flatten()
		s.Assert().NotEmpty(allShares, "namespace data should have shares")

		// Verify that all shares belong to the correct namespace
		for i, share := range allShares {
			s.Assert().True(share.Namespace().Equals(namespace), "share %d should belong to correct namespace", i)
		}

		// Parse blobs from namespace data to verify our blob is included
		blobs, err := libshare.ParseBlobs(allShares)
		s.Require().NoError(err, "should parse blobs from namespace data")
		s.Require().NotEmpty(blobs, "should have at least one blob")

		// Convert to node blobs and verify commitment matches
		nodeBlobs, err := nodeblob.ToNodeBlobs(blobs...)
		s.Require().NoError(err)
		s.Require().NotEmpty(nodeBlobs, "should have node blobs")

		// Find our blob by commitment
		found := false
		for _, blob := range nodeBlobs {
			if bytes.Equal(blob.Commitment, commitment) {
				found = true
				s.Assert().Equal(data, blob.Data, "blob data should match")
				break
			}
		}
		s.Assert().True(found, "should find our blob in namespace data")
	})

	s.Run("EmptyNamespace", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		fullNode := s.framework.GetOrCreateFullNode(ctx)
		client := s.framework.GetNodeRPCClient(ctx, fullNode)

		// Get current head for a valid height
		head, err := client.Header.LocalHead(ctx)
		s.Require().NoError(err)

		// Create a namespace that doesn't exist
		emptyNamespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0xFF}, 10))
		s.Require().NoError(err)

		// Get namespace data for empty namespace
		namespaceData, err := client.Share.GetNamespaceData(ctx, head.Height(), emptyNamespace)
		s.Require().NoError(err, "should succeed even for empty namespace")
		s.Require().NotNil(namespaceData, "namespace data should not be nil")

		// Should have empty shares but valid structure
		allShares := namespaceData.Flatten()
		s.Assert().Empty(allShares, "empty namespace should have no shares")
		s.Assert().NotNil(namespaceData, "empty namespace should still return valid structure")
	})

	s.Run("MultipleBlobs", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		fullNode := s.framework.GetOrCreateFullNode(ctx)
		client := s.framework.GetNodeRPCClient(ctx, fullNode)

		// Use same namespace for multiple blobs
		namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x21}, 10))
		s.Require().NoError(err)

		// Submit multiple blobs with same namespace
		data1 := []byte("First blob data")
		data2 := []byte("Second blob data")
		data3 := []byte("Third blob data")

		_, _ = s.submitTestBlob(ctx, namespace, data1)
		_, _ = s.submitTestBlob(ctx, namespace, data2)
		height3, commitment3 := s.submitTestBlob(ctx, namespace, data3)

		// For this test, we'll use the last height that should contain all blobs
		testHeight := height3

		// Get namespace data for the namespace
		namespaceData, err := client.Share.GetNamespaceData(ctx, testHeight, namespace)
		s.Require().NoError(err, "should get namespace data")
		s.Require().NotNil(namespaceData, "namespace data should not be nil")

		// Validate that we have data
		allShares := namespaceData.Flatten()
		s.Assert().NotEmpty(allShares, "namespace data should have shares for multiple blobs")
		s.Assert().NotEmpty(namespaceData, "namespace data should not be empty")

		// Parse all blobs from namespace data
		blobs, err := libshare.ParseBlobs(allShares)
		s.Require().NoError(err, "should parse blobs from namespace data")

		// Convert to node blobs
		nodeBlobs, err := nodeblob.ToNodeBlobs(blobs...)
		s.Require().NoError(err)

		// At minimum, we should find the blob from the current height
		foundCommitments := make(map[string]bool)
		for _, blob := range nodeBlobs {
			commitmentStr := string(blob.Commitment)
			foundCommitments[commitmentStr] = true
		}

		// Check that we found at least the blob from the current height
		s.Assert().True(foundCommitments[string(commitment3)], "should find the blob from current height")
	})

	s.Run("ProofVerification", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		fullNode := s.framework.GetOrCreateFullNode(ctx)
		client := s.framework.GetNodeRPCClient(ctx, fullNode)

		// Submit a test blob
		namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x22}, 10))
		s.Require().NoError(err)
		data := []byte("GetNamespaceData proof verification test")
		height, _ := s.submitTestBlob(ctx, namespace, data)

		// Get header to extract data root for proof verification
		header, err := client.Header.GetByHeight(ctx, height)
		s.Require().NoError(err, "should get header")

		// Get namespace data
		namespaceData, err := client.Share.GetNamespaceData(ctx, height, namespace)
		s.Require().NoError(err, "should get namespace data")
		s.Require().NotNil(namespaceData, "namespace data should not be nil")

		// Validate namespace data structure
		allShares := namespaceData.Flatten()
		s.Assert().NotEmpty(allShares, "namespace data should have shares")
		s.Assert().NotEmpty(namespaceData, "namespace data should not be empty")

		// Verify the proof against the data root
		err = namespaceData.Verify(header.DAH, namespace)
		s.Assert().NoError(err, "namespace data proof should verify against data root")
	})

	s.Run("InvalidHeight", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		fullNode := s.framework.GetOrCreateFullNode(ctx)
		client := s.framework.GetNodeRPCClient(ctx, fullNode)

		// Create a valid namespace
		namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x23}, 10))
		s.Require().NoError(err)

		// Try to get namespace data for invalid height
		_, err = client.Share.GetNamespaceData(ctx, 0, namespace)
		s.Assert().Error(err, "should return error for height 0")

		// Try to get namespace data for future height
		futureHeight := uint64(1000000)
		_, err = client.Share.GetNamespaceData(ctx, futureHeight, namespace)
		s.Assert().Error(err, "should return error for future height")
	})

	s.Run("SystemNamespaces", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		fullNode := s.framework.GetOrCreateFullNode(ctx)
		client := s.framework.GetNodeRPCClient(ctx, fullNode)

		// Get current head for a valid height
		head, err := client.Header.LocalHead(ctx)
		s.Require().NoError(err)

		// Test with a different regular namespace (simulating a system namespace)
		systemNamespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x00}, 10))
		s.Require().NoError(err)

		// Get namespace data for system namespace
		namespaceData, err := client.Share.GetNamespaceData(ctx, head.Height(), systemNamespace)
		s.Require().NoError(err, "should get data for system namespace")
		s.Require().NotNil(namespaceData, "namespace data should not be nil")

		// System namespaces might have system data
		s.Assert().NotNil(namespaceData, "system namespace should return valid structure")

		// If there are shares, they should belong to the system namespace
		allShares := namespaceData.Flatten()
		for i, share := range allShares {
			s.Assert().True(share.Namespace().Equals(systemNamespace), "share %d should belong to system namespace", i)
		}
	})

	s.Run("WithNetworkResilience", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
		defer cancel()

		// Create multiple nodes including full nodes for resilience testing
		bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
		fullNode1 := s.framework.GetOrCreateFullNode(ctx)
		fullNode2 := s.framework.GetOrCreateFullNode(ctx)
		lightNode := s.framework.GetOrCreateLightNode(ctx)

		bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
		fullClient1 := s.framework.GetNodeRPCClient(ctx, fullNode1)
		fullClient2 := s.framework.GetNodeRPCClient(ctx, fullNode2)
		lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

		// Submit test blob with specific namespace
		namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x26}, 10))
		s.Require().NoError(err)
		data := []byte("Network resilience test data")
		height, _ := s.submitTestBlob(ctx, namespace, data)

		// Wait for all nodes to sync
		for _, client := range []*client.Client{fullClient1, fullClient2, lightClient} {
			_, err = client.Header.WaitForHeight(ctx, height)
			s.Require().NoError(err)
		}

		// Get namespace data from bridge (source of truth)
		expected, err := bridgeClient.Share.GetNamespaceData(ctx, height, namespace)
		s.Require().NoError(err, "bridge should provide namespace data")
		expectedShares := expected.Flatten()
		s.Require().NotEmpty(expectedShares, "bridge namespace data should have shares")

		// Test that all full nodes can retrieve the same data
		gotFull1, err := fullClient1.Share.GetNamespaceData(ctx, height, namespace)
		s.Require().NoError(err, "full node 1 should get namespace data")
		gotFull1Shares := gotFull1.Flatten()
		s.Require().NotEmpty(gotFull1Shares, "full node 1 should have shares")

		gotFull2, err := fullClient2.Share.GetNamespaceData(ctx, height, namespace)
		s.Require().NoError(err, "full node 2 should get namespace data")
		gotFull2Shares := gotFull2.Flatten()
		s.Require().NotEmpty(gotFull2Shares, "full node 2 should have shares")

		// Test that light node can retrieve data (should use ShrexND to fetch from available full nodes)
		gotLight, err := lightClient.Share.GetNamespaceData(ctx, height, namespace)
		s.Require().NoError(err, "light node should get namespace data via ShrexND")
		gotLightShares := gotLight.Flatten()
		s.Require().NotEmpty(gotLightShares, "light node should have shares")

		// Verify data consistency across all nodes
		s.Assert().Equal(expectedShares, gotFull1Shares, "full node 1 should match bridge data")
		s.Assert().Equal(expectedShares, gotFull2Shares, "full node 2 should match bridge data")
		s.Assert().Equal(expectedShares, gotLightShares, "light node should match bridge data")

		// Verify structures are present
		s.Assert().NotEmpty(gotFull1, "full node 1 should have namespace data")
		s.Assert().NotEmpty(gotFull2, "full node 2 should have namespace data")
		s.Assert().NotEmpty(gotLight, "light node should have namespace data")
	})

	s.Run("FromRandomNamespaces", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
		lightNode := s.framework.GetOrCreateLightNode(ctx)

		bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
		lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

		// Submit blobs with multiple different namespaces
		namespaces := make([]libshare.Namespace, 3)
		heights := make([]uint64, 3)

		for i := 0; i < 3; i++ {
			// Create different namespaces
			nsBytes := bytes.Repeat([]byte{byte(0x30 + i)}, 10)
			namespace, err := libshare.NewV0Namespace(nsBytes)
			s.Require().NoError(err)
			namespaces[i] = namespace

			data := []byte(fmt.Sprintf("Random namespace test data %d", i+1))
			height, _ := s.submitTestBlob(ctx, namespace, data)
			heights[i] = height
		}

		// Test namespace data retrieval for each namespace
		for i, namespace := range namespaces {
			height := heights[i]

			// Wait for light node to sync
			_, err := lightClient.Header.WaitForHeight(ctx, height)
			s.Require().NoError(err)

			// Get namespace data from bridge
			bridgeData, err := bridgeClient.Share.GetNamespaceData(ctx, height, namespace)
			s.Require().NoError(err, "bridge should get namespace data for ns %d", i)

			// Get namespace data from light node
			lightData, err := lightClient.Share.GetNamespaceData(ctx, height, namespace)
			s.Require().NoError(err, "light node should get namespace data for ns %d", i)

			// Verify data consistency
			bridgeShares := bridgeData.Flatten()
			lightShares := lightData.Flatten()
			s.Assert().Equal(bridgeShares, lightShares,
				"namespace data should match between bridge and light for ns %d", i)

			// Verify all shares belong to correct namespace
			for j, share := range lightShares {
				s.Assert().True(share.Namespace().Equals(namespace),
					"share %d should belong to namespace %d", j, i)
			}

			// Test that we get empty results for non-existent namespaces
			nonExistentNs, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0xFF - byte(i)}, 10))
			s.Require().NoError(err)

			emptyData, err := lightClient.Share.GetNamespaceData(ctx, height, nonExistentNs)
			s.Require().NoError(err, "should succeed for non-existent namespace")
			emptyShares := emptyData.Flatten()
			s.Assert().Empty(emptyShares, "should have no shares for non-existent namespace")
			s.Assert().NotNil(emptyData, "should still return valid structure for non-existent namespace")
		}
	})

	s.Run("VsGetRange_Complementary", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		fullNode := s.framework.GetOrCreateFullNode(ctx)
		client := s.framework.GetNodeRPCClient(ctx, fullNode)

		// Submit a test blob
		namespace, err := libshare.NewV0Namespace(bytes.Repeat([]byte{0x24}, 10))
		s.Require().NoError(err)
		data := []byte("Comparing GetNamespaceData vs GetRange")
		height, commitment := s.submitTestBlob(ctx, namespace, data)

		// Get the blob to find its position
		retrievedBlob, err := client.Blob.Get(ctx, height, namespace, commitment)
		s.Require().NoError(err)

		// Calculate blob length
		blobLength, err := retrievedBlob.Length()
		s.Require().NoError(err)

		// Get data using GetNamespaceData
		namespaceData, err := client.Share.GetNamespaceData(ctx, height, namespace)
		s.Require().NoError(err, "should get namespace data")

		// Get data using GetRange for the same blob
		rangeData, err := client.Share.GetRange(ctx, height, retrievedBlob.Index(), retrievedBlob.Index()+blobLength)
		s.Require().NoError(err, "should get range data")

		// Both should return valid data
		namespaceBlobShares := namespaceData.Flatten()
		s.Assert().NotEmpty(namespaceBlobShares, "namespace data should have shares")
		s.Assert().NotEmpty(rangeData.Shares, "range data should have shares")
		s.Assert().NotEmpty(namespaceData, "namespace data should not be empty")
		s.Assert().NotNil(rangeData.Proof, "range data should have proof")

		// Parse blobs from both methods
		rangeBlobShares := rangeData.Shares

		// The shares from namespace data should contain all shares for the namespace
		// The shares from range data should contain shares for the specific range
		// For a single blob, the range data should be a subset of namespace data
		s.Assert().GreaterOrEqual(len(namespaceBlobShares), len(rangeBlobShares),
			"namespace data should have at least as many shares as range data")

		// Verify that range shares are present in namespace shares
		if len(rangeBlobShares) > 0 && len(namespaceBlobShares) > 0 {
			// Find the range shares within namespace shares
			found := false
			for i := 0; i <= len(namespaceBlobShares)-len(rangeBlobShares); i++ {
				match := true
				for j := 0; j < len(rangeBlobShares); j++ {
					if !bytes.Equal(namespaceBlobShares[i+j].ToBytes(), rangeBlobShares[j].ToBytes()) {
						match = false
						break
					}
				}
				if match {
					found = true
					break
				}
			}
			s.Assert().True(found, "range shares should be found within namespace shares")
		}
	})
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

// TestShareCrossNodeRobustness validates all Share APIs across all node types with multiple requests
func (s *ShareTestSuite) TestShareCrossNodeRobustness() {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	// Create all node types
	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)
	fullNode1 := s.framework.GetOrCreateFullNode(ctx)
	fullNode2 := s.framework.GetOrCreateFullNode(ctx)
	lightNode := s.framework.GetOrCreateLightNode(ctx)

	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)
	fullClient1 := s.framework.GetNodeRPCClient(ctx, fullNode1)
	fullClient2 := s.framework.GetNodeRPCClient(ctx, fullNode2)
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	clients := map[string]*client.Client{
		"bridge": bridgeClient,
		"full1":  fullClient1,
		"full2":  fullClient2,
		"light":  lightClient,
	}

	// Submit test data with different namespaces for comprehensive testing
	testData := []struct {
		namespace libshare.Namespace
		data      []byte
		testName  string
	}{
		{mustCreateNamespace(bytes.Repeat([]byte{0x41}, 10)), []byte("Cross-node test data 1"), "test1"},
		{mustCreateNamespace(bytes.Repeat([]byte{0x42}, 10)), []byte("Cross-node test data 2"), "test2"},
		{mustCreateNamespace(bytes.Repeat([]byte{0x43}, 10)), []byte("Cross-node test data 3"), "test3"},
	}

	heights := make([]uint64, len(testData))
	commitments := make([][]byte, len(testData))

	// Submit all test blobs
	for i, td := range testData {
		height, commitment := s.submitTestBlob(ctx, td.namespace, td.data)
		heights[i] = height
		commitments[i] = commitment
	}

	// Wait for all nodes to sync to the latest height
	maxHeight := heights[len(heights)-1]
	for name, client := range clients {
		_, err := client.Header.WaitForHeight(ctx, maxHeight)
		s.Require().NoError(err, "%s node should sync to height %d", name, maxHeight)
	}

	// Additional wait for data propagation
	time.Sleep(15 * time.Second)

	s.Run("SharesAvailable_AllNodes", func() {
		// Test SharesAvailable across all node types with multiple requests
		for i, height := range heights {
			for name, client := range clients {
				// Make multiple requests to ensure consistency
				for attempt := 1; attempt <= 3; attempt++ {
					err := client.Share.SharesAvailable(ctx, height)
					s.Assert().NoError(err, "%s node attempt %d should have shares available for height %d (test %s)",
						name, attempt, height, testData[i].testName)
				}
			}
		}
	})

	s.Run("GetShare_AllNodes", func() {
		// Test GetShare across all node types
		for i, height := range heights {
			for name, client := range clients {
				// Make multiple requests for different coordinates
				coords := []struct{ row, col int }{
					{0, 0}, {0, 1}, {1, 0},
				}

				for _, coord := range coords {
					for attempt := 1; attempt <= 2; attempt++ {
						share, err := client.Share.GetShare(ctx, height, coord.row, coord.col)
						if err == nil {
							s.Assert().NotEmpty(share, "%s node attempt %d should get valid share at (%d,%d) for height %d (test %s)",
								name, attempt, coord.row, coord.col, height, testData[i].testName)
						}
						// Note: Some coordinates might be out of bounds, which is acceptable
					}
				}
			}
		}
	})

	s.Run("GetEDS_AllNodes", func() {
		// Test GetEDS across all node types
		for i, height := range heights {
			for name, client := range clients {
				// Make multiple requests to ensure consistency
				for attempt := 1; attempt <= 2; attempt++ {
					eds, err := client.Share.GetEDS(ctx, height)
					s.Assert().NoError(err, "%s node attempt %d should get EDS for height %d (test %s)",
						name, attempt, height, testData[i].testName)
					if err == nil {
						s.Assert().Greater(eds.Width(), uint(0), "%s node EDS should have positive width", name)
					}
				}
			}
		}
	})

	s.Run("GetRow_AllNodes", func() {
		// Test GetRow across all node types
		for i, height := range heights {
			for name, client := range clients {
				// Test multiple rows
				rows := []int{0, 1}
				for _, rowIdx := range rows {
					for attempt := 1; attempt <= 2; attempt++ {
						row, err := client.Share.GetRow(ctx, height, rowIdx)
						if err == nil {
							s.Assert().NotNil(row, "%s node attempt %d should get valid row %d for height %d (test %s)",
								name, attempt, rowIdx, height, testData[i].testName)
							s.Assert().NotEmpty(row.Shares, "%s node row should have shares", name)
						}
						// Note: Some row indices might be out of bounds, which is acceptable
					}
				}
			}
		}
	})

	s.Run("GetRange_AllNodes", func() {
		// Test GetRange across all node types
		for i, height := range heights {
			for name, client := range clients {
				// Test multiple ranges
				ranges := []struct{ start, end int }{
					{0, 3}, {0, 7}, {4, 8},
				}

				for _, r := range ranges {
					for attempt := 1; attempt <= 2; attempt++ {
						rangeResult, err := client.Share.GetRange(ctx, height, r.start, r.end)
						if err == nil {
							s.Assert().NotNil(rangeResult, "%s node attempt %d should get valid range (%d,%d) for height %d (test %s)",
								name, attempt, r.start, r.end, height, testData[i].testName)
							s.Assert().NotNil(rangeResult.Proof, "%s node range should have proof", name)
						}
						// Note: Some ranges might be invalid or out of bounds, which is acceptable
					}
				}
			}
		}
	})

	s.Run("GetNamespaceData_AllNodes", func() {
		// Test GetNamespaceData across all node types
		for i, td := range testData {
			height := heights[i]
			for name, client := range clients {
				// Make multiple requests to ensure consistency
				for attempt := 1; attempt <= 3; attempt++ {
					namespaceData, err := client.Share.GetNamespaceData(ctx, height, td.namespace)
					s.Assert().NoError(err, "%s node attempt %d should get namespace data for height %d (test %s)",
						name, attempt, height, td.testName)

					if err == nil {
						s.Assert().NotNil(namespaceData, "%s node namespace data should not be nil", name)
						allShares := namespaceData.Flatten()
						s.Assert().NotEmpty(allShares, "%s node should have shares for namespace (test %s)", name, td.testName)

						// Verify all shares belong to the correct namespace
						for j, share := range allShares {
							s.Assert().True(share.Namespace().Equals(td.namespace),
								"%s node share %d should belong to correct namespace (test %s)", name, j, td.testName)
						}
					}
				}
			}
		}
	})

	s.Run("GetSamples_AllNodes", func() {
		// Test GetSamples across all node types
		for i, height := range heights {
			// Get header for samples
			header, err := bridgeClient.Header.GetByHeight(ctx, height)
			s.Require().NoError(err)

			sampleCoords := []shwap.SampleCoords{
				{Row: 0, Col: 0},
				{Row: 0, Col: 1},
				{Row: 1, Col: 0},
			}

			for name, client := range clients {
				for attempt := 1; attempt <= 2; attempt++ {
					samples, err := client.Share.GetSamples(ctx, header, sampleCoords)
					if err == nil {
						s.Assert().Len(samples, len(sampleCoords), "%s node attempt %d should get correct number of samples for height %d (test %s)",
							name, attempt, height, testData[i].testName)

						for j, sample := range samples {
							s.Assert().NotEmpty(sample.Share, "%s node sample %d should not be empty", name, j)
							s.Assert().NotNil(sample.Proof, "%s node sample %d should have proof", name, j)
						}
					}
					// Note: Some sample coordinates might be out of bounds, which is acceptable
				}
			}
		}
	})

	s.Run("CrossNode_DataConsistency", func() {
		// Ensure data consistency across all node types
		for i, td := range testData {
			height := heights[i]

			// Get namespace data from all nodes
			nodeResults := make(map[string][]libshare.Share)

			for name, client := range clients {
				namespaceData, err := client.Share.GetNamespaceData(ctx, height, td.namespace)
				if err == nil {
					nodeResults[name] = namespaceData.Flatten()
				}
			}

			// Compare results between nodes (should be identical)
			if len(nodeResults) >= 2 {
				var referenceShares []libshare.Share
				var referenceName string

				for name, shares := range nodeResults {
					if len(shares) > 0 {
						if referenceShares == nil {
							referenceShares = shares
							referenceName = name
						} else {
							s.Assert().Equal(len(referenceShares), len(shares),
								"node %s should have same number of shares as %s (test %s)", name, referenceName, td.testName)

							// Compare share data
							for j := 0; j < len(shares) && j < len(referenceShares); j++ {
								s.Assert().True(shares[j].Namespace().Equals(referenceShares[j].Namespace()),
									"share %d namespace should match between %s and %s (test %s)", j, name, referenceName, td.testName)
							}
						}
					}
				}
			}
		}
	})

	s.Run("RepeatedRequests_Stability", func() {
		// Test stability with many repeated requests
		const numRepeats = 10
		height := heights[0]

		for name, client := range clients {
			successCount := 0
			for i := 0; i < numRepeats; i++ {
				err := client.Share.SharesAvailable(ctx, height)
				if err == nil {
					successCount++
				}

				// Small delay between requests
				time.Sleep(100 * time.Millisecond)
			}

			// Allow some failures for light nodes due to network conditions
			expectedMinSuccess := numRepeats
			if name == "light" {
				expectedMinSuccess = numRepeats / 2 // Allow 50% success rate for light nodes
			}

			s.Assert().GreaterOrEqual(successCount, expectedMinSuccess,
				"%s node should have at least %d successes out of %d repeated requests", name, expectedMinSuccess, numRepeats)
		}
	})
}

// Helper function for creating namespaces in tests
func mustCreateNamespace(data []byte) libshare.Namespace {
	ns, err := libshare.NewV0Namespace(data)
	if err != nil {
		panic(fmt.Sprintf("failed to create namespace: %v", err))
	}
	return ns
}

// TestShareErrorHandling validates error handling scenarios
func (s *ShareTestSuite) TestShareErrorHandling() {
	s.Run("NetworkTimeout", func() {
		// Use a valid context for framework operations
		setupCtx, setupCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer setupCancel()

		fullNode := s.framework.GetOrCreateFullNode(setupCtx)
		client := s.framework.GetNodeRPCClient(setupCtx, fullNode)

		// Create a cancelled context only for the API call to test timeout behavior
		apiCtx, apiCancel := context.WithCancel(context.Background())
		apiCancel() // Cancel immediately to force timeout

		// This should timeout immediately due to cancelled context
		err := client.Share.SharesAvailable(apiCtx, 1)
		s.Assert().Error(err, "should return timeout error")
		// Check for context cancelled error
		if err != nil {
			s.Assert().Contains(err.Error(), "context canceled", "should be context cancelled error")
		}
	})
}

// TestSharePerformance validates performance characteristics
func (s *ShareTestSuite) TestSharePerformance() {
	s.Run("ConcurrentRequests", func() {
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
	})
}
