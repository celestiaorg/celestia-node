//go:build integration

package tastora

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	libhead "github.com/celestiaorg/go-header"
)

// HeaderTestSuite provides comprehensive testing of the Header module APIs.
// Tests header synchronization, retrieval, and validation functionality.
type HeaderTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestHeaderTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Header module integration tests in short mode")
	}
	suite.Run(t, &HeaderTestSuite{})
}

func (s *HeaderTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithFullNodes(1), WithBridgeNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

// TestHeaderLocalHead validates LocalHead API functionality
func (s *HeaderTestSuite) TestHeaderLocalHead_Success() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get local head
	localHead, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should retrieve local head")
	s.Require().NotNil(localHead, "local head should not be nil")

	// Validate header structure
	s.Assert().Greater(localHead.Height(), uint64(0), "local head height should be greater than 0")
	s.Assert().NotEmpty(localHead.Hash(), "local head hash should not be empty")
	s.Assert().NotEmpty(localHead.ChainID(), "local head chain ID should not be empty")
	s.Assert().NotZero(localHead.Time(), "local head time should not be zero")
}

// TestHeaderNetworkHead validates NetworkHead API functionality
func (s *HeaderTestSuite) TestHeaderNetworkHead_Success() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get network head
	networkHead, err := client.Header.NetworkHead(ctx)
	s.Require().NoError(err, "should retrieve network head")
	s.Require().NotNil(networkHead, "network head should not be nil")

	// Validate header structure
	s.Assert().Greater(networkHead.Height(), uint64(0), "network head height should be greater than 0")
	s.Assert().NotEmpty(networkHead.Hash(), "network head hash should not be empty")
	s.Assert().NotEmpty(networkHead.ChainID(), "network head chain ID should not be empty")
	s.Assert().NotZero(networkHead.Time(), "network head time should not be zero")
}

// TestHeaderLocalHeadVsNetworkHead validates consistency between LocalHead and NetworkHead
func (s *HeaderTestSuite) TestHeaderLocalHeadVsNetworkHead_Consistency() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Compare local vs network head
	localHead, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should get local head")

	networkHead, err := client.Header.NetworkHead(ctx)
	s.Require().NoError(err, "should get network head")

	// Heights should be close (within reasonable difference)
	heightDiff := int64(networkHead.Height()) - int64(localHead.Height())
	s.Assert().LessOrEqual(heightDiff, int64(10), "local and network head should be close")

	// Chain IDs should match
	s.Assert().Equal(networkHead.ChainID(), localHead.ChainID(), "chain IDs should match")

	// Wait for local head to catch up more
	time.Sleep(5 * time.Second)

	localHead2, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should get updated local head")

	// Local head should have progressed
	s.Assert().GreaterOrEqual(localHead2.Height(), localHead.Height(), "local head should progress")

	// Updated network head
	networkHead2, err := client.Header.NetworkHead(ctx)
	s.Require().NoError(err, "should get updated network head")

	heightDiff2 := int64(networkHead2.Height()) - int64(localHead2.Height())
	s.Assert().LessOrEqual(heightDiff2, int64(10), "local and network head should remain close")
}

// TestHeaderGetByHeightVsGetByHash validates consistency between GetByHeight and GetByHash
func (s *HeaderTestSuite) TestHeaderGetByHeightVsGetByHash_Consistency() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get current head to ensure we have a valid height
	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should retrieve local head")

	// Get header by height
	headerByHeight, err := client.Header.GetByHeight(ctx, head.Height())
	s.Require().NoError(err, "should retrieve header by height")

	// Get same header by hash
	headerByHash, err := client.Header.GetByHash(ctx, headerByHeight.Hash())
	s.Require().NoError(err, "should retrieve header by hash")

	// Headers should be identical
	s.Assert().Equal(headerByHeight.Height(), headerByHash.Height(), "heights should match")
	s.Assert().Equal(headerByHeight.Hash(), headerByHash.Hash(), "hashes should match")
	s.Assert().Equal(headerByHeight.ChainID(), headerByHash.ChainID(), "chain IDs should match")
	s.Assert().Equal(headerByHeight.Time(), headerByHash.Time(), "timestamps should match")

	// Test with an older block to ensure consistency across heights
	if head.Height() > 2 {
		olderHeight := head.Height() - 1
		olderByHeight, err := client.Header.GetByHeight(ctx, olderHeight)
		s.Require().NoError(err, "should get older header by height")

		olderByHash, err := client.Header.GetByHash(ctx, olderByHeight.Hash())
		s.Require().NoError(err, "should get older header by hash")

		s.Assert().Equal(olderByHeight.Height(), olderByHash.Height(), "older headers should match by height")
		s.Assert().Equal(olderByHeight.Hash(), olderByHash.Hash(), "older headers should match by hash")
	}
}

// TestHeaderFieldIntegrity validates header field consistency and structure
func (s *HeaderTestSuite) TestHeaderFieldIntegrity() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get current head
	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should retrieve local head")

	// Validate header fields
	s.Assert().Greater(head.Height(), uint64(0), "height should be positive")
	s.Assert().NotEmpty(head.Hash(), "hash should not be empty")
	s.Assert().Len(head.Hash(), 32, "hash should be 32 bytes")
	s.Assert().NotEmpty(head.ChainID(), "chain ID should not be empty")
	s.Assert().NotZero(head.Time(), "timestamp should not be zero")

	// Test sequential headers for consistency
	if head.Height() > 1 {
		prevHeader, err := client.Header.GetByHeight(ctx, head.Height()-1)
		s.Require().NoError(err, "should get previous header")

		// Sequential headers should have incremental heights
		s.Assert().Equal(head.Height()-1, prevHeader.Height(), "previous header height should be head-1")

		// Timestamps should be monotonic (current >= previous)
		s.Assert().True(head.Time().After(prevHeader.Time()) || head.Time().Equal(prevHeader.Time()),
			"timestamps should be monotonic")

		// Chain IDs should match
		s.Assert().Equal(head.ChainID(), prevHeader.ChainID(), "chain IDs should be consistent")

		// Hashes should be different (unless fork, but unlikely in test)
		s.Assert().NotEqual(head.Hash(), prevHeader.Hash(), "consecutive headers should have different hashes")
	}
}

// TestHeaderGetByHeight validates GetByHeight API functionality
func (s *HeaderTestSuite) TestHeaderGetByHeight_ValidHeight() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get current head to get a valid height
	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should retrieve local head")

	// Get header by height
	headerByHeight, err := client.Header.GetByHeight(ctx, head.Height())
	s.Require().NoError(err, "should retrieve header by height")
	s.Require().NotNil(headerByHeight, "header by height should not be nil")

	// Validate retrieved header
	s.Assert().Equal(head.Height(), headerByHeight.Height(), "heights should match")
	s.Assert().Equal(head.Hash(), headerByHeight.Hash(), "hashes should match")
}

func (s *HeaderTestSuite) TestHeaderGetByHeight_InvalidHeight() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Try to get header at height 0 (invalid)
	_, err := client.Header.GetByHeight(ctx, 0)
	s.Assert().Error(err, "should return error for height 0")
	s.Assert().Contains(err.Error(), "height is equal to 0", "should be height zero error")
}

func (s *HeaderTestSuite) TestHeaderGetByHeight_FutureHeight() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get current head
	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should retrieve local head")

	// Try to get header from far future
	futureHeight := head.Height() + 1000
	_, err = client.Header.GetByHeight(ctx, futureHeight)
	s.Assert().Error(err, "should return error for future height")
	s.Assert().Contains(err.Error(), "from the future", "should be future height error")
}

// TestHeaderGetByHash validates GetByHash API functionality
func (s *HeaderTestSuite) TestHeaderGetByHash_ValidHash() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get current head to get a valid hash
	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should retrieve local head")

	// Get header by hash
	headerByHash, err := client.Header.GetByHash(ctx, head.Hash())
	s.Require().NoError(err, "should retrieve header by hash")
	s.Require().NotNil(headerByHash, "header by hash should not be nil")

	// Validate retrieved header
	s.Assert().Equal(head.Height(), headerByHash.Height(), "heights should match")
	s.Assert().Equal(head.Hash(), headerByHash.Hash(), "hashes should match")
}

func (s *HeaderTestSuite) TestHeaderGetByHash_InvalidHash() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Try to get header with invalid hash
	invalidHash := libhead.Hash(make([]byte, 32)) // zero hash
	_, err := client.Header.GetByHash(ctx, invalidHash)
	s.Assert().Error(err, "should return error for invalid hash")
}

func (s *HeaderTestSuite) TestHeaderGetByHash_EmptyHash() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Try to get header with empty hash
	emptyHash := libhead.Hash([]byte{})
	_, err := client.Header.GetByHash(ctx, emptyHash)
	s.Assert().Error(err, "should return error for empty hash")
}

func (s *HeaderTestSuite) TestHeaderGetByHash_WrongLengthHash() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Try to get header with wrong length hash
	wrongLengthHash := libhead.Hash(make([]byte, 16)) // wrong length
	_, err := client.Header.GetByHash(ctx, wrongLengthHash)
	s.Assert().Error(err, "should return error for wrong length hash")
}

// TestHeaderWaitForHeight validates WaitForHeight API functionality
func (s *HeaderTestSuite) TestHeaderWaitForHeight_CurrentHeight() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get current head
	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should retrieve local head")

	// Wait for current height (should return immediately)
	headerAtHeight, err := client.Header.WaitForHeight(ctx, head.Height())
	s.Require().NoError(err, "should wait for current height")
	s.Require().NotNil(headerAtHeight, "header at height should not be nil")

	s.Assert().Equal(head.Height(), headerAtHeight.Height(), "heights should match")
}

func (s *HeaderTestSuite) TestHeaderWaitForHeight_FutureHeight() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get current head
	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should retrieve local head")

	// Wait for next height (should wait for new block)
	nextHeight := head.Height() + 1
	headerAtHeight, err := client.Header.WaitForHeight(ctx, nextHeight)
	s.Require().NoError(err, "should wait for next height")
	s.Require().NotNil(headerAtHeight, "header at height should not be nil")

	s.Assert().Equal(nextHeight, headerAtHeight.Height(), "should get header at next height")
}

func (s *HeaderTestSuite) TestHeaderWaitForHeight_Timeout() {
	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get current head
	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should retrieve local head")

	// Wait for far future height with short timeout
	futureHeight := head.Height() + 100
	_, err = client.Header.WaitForHeight(ctx, futureHeight)
	s.Assert().Error(err, "should timeout waiting for future height")
	s.Assert().Contains(err.Error(), "context deadline exceeded", "should be timeout error")
}

func (s *HeaderTestSuite) TestHeaderWaitForHeight_InvalidHeight() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Try to wait for height 0 (invalid)
	_, err := client.Header.WaitForHeight(ctx, 0)
	s.Assert().Error(err, "should return error for height 0")
	s.Assert().Contains(err.Error(), "height must be bigger than zero", "should be height zero error")
}

// TestHeaderGetRangeByHeight validates GetRangeByHeight API functionality
func (s *HeaderTestSuite) TestHeaderGetRangeByHeight_ValidRange() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Wait for enough blocks to exist
	_, err := client.Header.WaitForHeight(ctx, 6)
	s.Require().NoError(err, "should wait for enough blocks")

	// GetRangeByHeight is exclusive of both from and to heights: [from+1:to)
	// To get headers at heights 2, 3, 4 we need:
	// - fromHeader at height 1 (so we get headers starting from height 2)
	// - toHeight at 5 (so we get headers up to but not including height 5)
	fromHeight := uint64(1)
	toHeight := uint64(5)

	fromHeader, err := client.Header.GetByHeight(ctx, fromHeight)
	s.Require().NoError(err, "should get from header")

	headers, err := client.Header.GetRangeByHeight(ctx, fromHeader, toHeight)
	s.Require().NoError(err, "should retrieve header range")
	s.Require().NotEmpty(headers, "header range should not be empty")

	// Validate range - should get 3 headers (heights 2, 3, 4)
	// Range is [fromHeight+1, toHeight) = [2, 5) = [2, 3, 4]
	expectedCount := 3
	s.Assert().Equal(expectedCount, len(headers), "should get correct number of headers")

	// Validate headers are in order starting from fromHeight + 1
	for i, header := range headers {
		expectedHeight := fromHeight + 1 + uint64(i) // Start from height 2
		s.Assert().Equal(expectedHeight, header.Height(), "headers should be in order")
	}
}

func (s *HeaderTestSuite) TestHeaderGetRangeByHeight_EmptyRange() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Wait for enough blocks to exist
	_, err := client.Header.WaitForHeight(ctx, 5)
	s.Require().NoError(err, "should wait for enough blocks")

	fromHeader, err := client.Header.GetByHeight(ctx, 4)
	s.Require().NoError(err, "should get from header")

	// Empty range: from height 4, to height 5
	// GetRangeByHeight API validates that to > from, so this should return an error
	_, err = client.Header.GetRangeByHeight(ctx, fromHeader, 5)
	s.Assert().Error(err, "should return error for empty range where to <= from+1")
	s.Assert().Contains(err.Error(), "invalid range", "should be invalid range error")
}

func (s *HeaderTestSuite) TestHeaderGetRangeByHeight_SingleHeader() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Wait for enough blocks to exist
	_, err := client.Header.WaitForHeight(ctx, 4)
	s.Require().NoError(err, "should wait for enough blocks")

	fromHeader, err := client.Header.GetByHeight(ctx, 2)
	s.Require().NoError(err, "should get from header")

	// Single header range: from height 2, to height 4 -> should return header at height 3
	headers, err := client.Header.GetRangeByHeight(ctx, fromHeader, 4)
	s.Require().NoError(err, "should get single header range")
	s.Assert().Len(headers, 1, "should return exactly one header")
	s.Assert().Equal(uint64(3), headers[0].Height(), "should return header at height 3")
}

func (s *HeaderTestSuite) TestHeaderGetRangeByHeight_InvalidRange() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Wait for enough blocks to exist
	_, err := client.Header.WaitForHeight(ctx, 5)
	s.Require().NoError(err, "should wait for enough blocks")

	fromHeader, err := client.Header.GetByHeight(ctx, 4)
	s.Require().NoError(err, "should get from header")

	// Invalid range: to height is before from height
	_, err = client.Header.GetRangeByHeight(ctx, fromHeader, 2)
	s.Assert().Error(err, "should return error for invalid range where to < from")
}

func (s *HeaderTestSuite) TestHeaderGetRangeByHeight_FutureRange() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get current head
	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should get current head")

	fromHeader, err := client.Header.GetByHeight(ctx, head.Height())
	s.Require().NoError(err, "should get from header")

	// Try range extending into future
	futureHeight := head.Height() + 100
	_, err = client.Header.GetRangeByHeight(ctx, fromHeader, futureHeight)
	s.Assert().Error(err, "should return error for range extending into future")
}

// TestHeaderSyncState validates SyncState API functionality
func (s *HeaderTestSuite) TestHeaderSyncState_Success() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Check sync state
	syncState, err := client.Header.SyncState(ctx)
	s.Require().NoError(err, "should get sync state")
	s.Assert().NotNil(syncState, "sync state should not be nil")
	s.Assert().GreaterOrEqual(syncState.Height, uint64(1), "sync state should have height")
	s.Assert().True(syncState.Finished(), "sync should be finished or progressing")
}

func (s *HeaderTestSuite) TestHeaderSyncState_Details() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get sync state and validate details
	syncState, err := client.Header.SyncState(ctx)
	s.Require().NoError(err, "should get sync state")

	// Get current head for comparison
	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should get local head")

	// Sync state height should be at least as high as local head
	s.Assert().GreaterOrEqual(syncState.Height, head.Height(), "sync state height should be at least local head height")

	// Since we're in a test environment, sync should be caught up quickly
	s.Assert().True(syncState.Finished(), "sync should be finished in test environment")
}

// TestHeaderSyncWait validates SyncWait API functionality
func (s *HeaderTestSuite) TestHeaderSyncWait_Success() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// SyncWait should complete (node should already be synced)
	err := client.Header.SyncWait(ctx)
	s.Require().NoError(err, "sync wait should complete successfully")
}

// TestHeaderSubscribe validates Subscribe API functionality
func (s *HeaderTestSuite) TestHeaderSubscribe_ReceiveHeaders() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Try to subscribe to headers
	_, err := client.Header.Subscribe(ctx)

	// The Subscribe API is not supported over JSON-RPC (no out channel support)
	// This is expected behavior, so we test that we get the correct error
	s.Assert().Error(err, "Subscribe should not be supported over JSON-RPC")
	s.Assert().Contains(err.Error(), "not supported in this mode", "should be unsupported mode error")
	s.Assert().Contains(err.Error(), "no out channel support", "should mention channel support limitation")
}

// TestHeaderErrorHandling validates error handling scenarios
func (s *HeaderTestSuite) TestHeaderErrorHandling_NetworkTimeout() {
	// Create an already cancelled context to guarantee timeout behavior
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to force timeout

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// This should timeout immediately due to cancelled context
	_, err := client.Header.LocalHead(ctx)
	s.Assert().Error(err, "should return timeout error")
	// Check for context cancelled error instead of deadline exceeded
	if err != nil {
		s.Assert().Contains(err.Error(), "context canceled", "should be context cancelled error")
	}
}

// TestHeaderCrossNodeConsistency validates header consistency across nodes
func (s *HeaderTestSuite) TestHeaderCrossNodeConsistency() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetOrCreateFullNode(ctx)
	bridgeNode := s.framework.GetOrCreateBridgeNode(ctx)

	fullClient := s.framework.GetNodeRPCClient(ctx, fullNode)
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Wait for both nodes to sync
	err := fullClient.Header.SyncWait(ctx)
	s.Require().NoError(err, "full node should sync")

	err = bridgeClient.Header.SyncWait(ctx)
	s.Require().NoError(err, "bridge node should sync")

	// Get heads from both nodes
	fullHead, err := fullClient.Header.LocalHead(ctx)
	s.Require().NoError(err, "should get full node head")

	bridgeHead, err := bridgeClient.Header.LocalHead(ctx)
	s.Require().NoError(err, "should get bridge node head")

	// Heights should be close (within reasonable sync tolerance)
	heightDiff := int64(bridgeHead.Height()) - int64(fullHead.Height())
	s.Assert().LessOrEqual(heightDiff, int64(5), "nodes should be within sync tolerance")
	s.Assert().GreaterOrEqual(heightDiff, int64(-5), "nodes should be within sync tolerance")
}
