//go:build integration

package tastora

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/state"
	"github.com/celestiaorg/go-square/v3/share"
)

// StateQueuedSubmissionTestSuite provides comprehensive testing of the queued submission feature.
// Tests direct submission, queued submission, and parallel submission scenarios.
type StateQueuedSubmissionTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestStateQueuedSubmissionTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping State queued submission integration tests in short mode")
	}
	suite.Run(t, &StateQueuedSubmissionTestSuite{})
}

func (s *StateQueuedSubmissionTestSuite) SetupSuite() {
	// Use a longer timeout for initial setup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Create framework with default configuration (direct submission)
	s.framework = NewFramework(s.T(), WithValidators(1), WithTxWorkerAccounts(0))
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

// TestDirectSubmission tests direct submission (TxWorkerAccounts = 0 - default case)
// This test submits blobs rapidly without any queuing mechanism
// Expected: WILL MISS/DROP blobs due to sequence conflicts from rapid submission
func (s *StateQueuedSubmissionTestSuite) TestDirectSubmission() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Use the shared framework (already configured for direct submission)
	bridgeNode := s.framework.GetBridgeNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Test rapid submission - direct mode WILL have sequence conflicts and miss blobs
	successCount := 0
	failureCount := 0
	totalBlobs := 8 // More blobs to increase chance of sequence conflicts
	startTime := time.Now()

	// Submit all blobs rapidly without waiting (testing immediate submission)
	for i := 0; i < totalBlobs; i++ {
		blob := s.createTestBlob(ctx, client, i)
		txConfig := s.createTxConfig()

		// Submit blob immediately - direct mode WILL have sequence conflicts
		response, err := s.submitBlobViaJSONRPC(ctx, client, blob, txConfig)
		if err == nil && s.isSuccessfulResponse(response) {
			successCount++
			s.T().Logf("Direct submission blob %d successful (immediate)", i+1)
		} else {
			failureCount++
			s.T().Logf("Direct submission blob %d FAILED (sequence conflict): %v", i+1, err)
		}
	}

	// Measure actual submission time
	submissionElapsed := time.Since(startTime)

	// Direct submission WILL have failures due to sequence conflicts
	s.Require().GreaterOrEqual(successCount, 1, "At least 1 blob should be submitted successfully with direct submission")
	s.Require().Greater(failureCount, 0, "Direct submission SHOULD have failures due to sequence conflicts - this proves the problem!")
	s.T().Logf("Direct submission: %d/%d blobs successful, %d FAILED (sequence conflicts) in %v", successCount, totalBlobs, failureCount, submissionElapsed)
}

// TestQueuedSubmission tests queued submission with 1 TxWorkerAccounts
// This test submits blobs rapidly (same as direct submission) but with queuing enabled
// Expected: NO MISSED/DROPPED blobs due to proper queuing handling sequence conflicts
// Note: This will be SLOWER than parallel submission (1 lane vs 8 lanes)
func (s *StateQueuedSubmissionTestSuite) TestQueuedSubmission() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create framework with queued submission (TxWorkerAccounts = 1)
	framework := NewFramework(s.T(), WithValidators(1), WithTxWorkerAccounts(1))
	s.Require().NoError(framework.SetupNetwork(ctx))

	bridgeNode := framework.GetBridgeNodes()[0]
	client := framework.GetNodeRPCClient(ctx, bridgeNode)

	// Test rapid submission with queuing - should NEVER miss blobs due to proper queuing
	successCount := 0
	failureCount := 0
	totalBlobs := 8 // Same number as direct submission for fair comparison
	startTime := time.Now()

	// Submit blobs rapidly (same as direct submission) but with queuing enabled
	for i := 0; i < totalBlobs; i++ {
		blob := s.createTestBlob(ctx, client, i)
		txConfig := s.createTxConfig()

		// Submit blob rapidly - queued mode should NEVER have sequence conflicts
		response, err := s.submitBlobViaJSONRPC(ctx, client, blob, txConfig)
		if err == nil && s.isSuccessfulResponse(response) {
			successCount++
			s.T().Logf("Queued submission blob %d successful (rapid with queuing)", i+1)
		} else {
			failureCount++
			s.T().Logf("Queued submission blob %d FAILED (unexpected): %v", i+1, err)
		}
	}

	// Measure actual submission time
	submissionElapsed := time.Since(startTime)

	// Queued submission should NEVER miss blobs - this proves queuing works!
	s.Require().Equal(successCount, totalBlobs, "Queued submission should NEVER miss blobs - all %d should be successful!", totalBlobs)
	s.Require().Equal(failureCount, 0, "Queued submission should have ZERO failures - this proves queuing prevents sequence conflicts!")
	s.T().Logf("Queued submission: %d/%d blobs successful, %d FAILED (perfect success!) in %v", successCount, totalBlobs, failureCount, submissionElapsed)
}

// TestParallelSubmission tests parallel submission with 8 TxWorkerAccounts
// This test demonstrates the SPEED advantage of parallel submission vs queued submission
// Expected: MUCH FASTER submission time due to 8 parallel lanes vs 1 queued lane
func (s *StateQueuedSubmissionTestSuite) TestParallelSubmission() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	// Create framework with parallel submission (TxWorkerAccounts = 8)
	framework := NewFramework(s.T(), WithValidators(1), WithTxWorkerAccounts(8))
	s.Require().NoError(framework.SetupNetwork(ctx))

	bridgeNode := framework.GetBridgeNodes()[0]
	client := framework.GetNodeRPCClient(ctx, bridgeNode)

	// Test concurrent submission - parallel mode should be MUCH FASTER than queued
	totalBlobs := 8 // Same number as queued for fair speed comparison
	successCount := 0
	failureCount := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	startTime := time.Now()

	// Submit blobs concurrently (testing parallel lanes) - should be MUCH FASTER than sequential
	for i := 0; i < totalBlobs; i++ {
		wg.Add(1)
		go func(blobIndex int) {
			defer wg.Done()

			blob := s.createTestBlob(ctx, client, blobIndex)
			txConfig := s.createTxConfig()

			// Submit blob using JSON-RPC (parallel lanes should handle this)
			response, err := s.submitBlobViaJSONRPC(ctx, client, blob, txConfig)
			if err == nil && s.isSuccessfulResponse(response) {
				mu.Lock()
				successCount++
				mu.Unlock()
				s.T().Logf("Parallel submission blob %d successful (concurrent)", blobIndex+1)
			} else {
				mu.Lock()
				failureCount++
				mu.Unlock()
				s.T().Logf("Parallel submission blob %d failed: %v", blobIndex+1, err)
			}
		}(i)
	}

	// Wait for all submissions to complete
	wg.Wait()
	submissionElapsed := time.Since(startTime)

	// Parallel submission should be MUCH FASTER than queued (8 lanes vs 1 lane)
	s.Require().Equal(successCount, totalBlobs, "Parallel submission should NEVER miss blobs - all %d should be successful!", totalBlobs)
	s.Require().Equal(failureCount, 0, "Parallel submission should have ZERO failures!")
	s.Require().Less(submissionElapsed, 2*time.Second, "Parallel submission should be MUCH FASTER than queued (8 lanes vs 1 lane) - should complete in <2 seconds!")
	s.T().Logf("Parallel submission: %d/%d blobs successful, %d FAILED in %v (SPEED ADVANTAGE: 8 parallel lanes!)", successCount, totalBlobs, failureCount, submissionElapsed)
}

// Helper methods

// createTestBlob creates a test blob with unique data
func (s *StateQueuedSubmissionTestSuite) createTestBlob(ctx context.Context, client *rpcclient.Client, index int) *share.Blob {
	// Create namespace with proper format (10 bytes for version 0)
	namespaceBytes := make([]byte, 10)
	// Fill with test data
	for i := 0; i < 10; i++ {
		namespaceBytes[i] = byte(index + i)
	}

	// Create namespace from bytes
	namespace, err := share.NewV0Namespace(namespaceBytes)
	s.Require().NoError(err, "Failed to create namespace")

	// Create test data
	data := []byte(fmt.Sprintf("test blob data %d - queued submission test", index))

	// Get the actual node address as the signer
	nodeAddr, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err, "Failed to get node address")

	blob, err := share.NewV1Blob(namespace, data, nodeAddr.Bytes())
	s.Require().NoError(err, "Failed to create test blob")
	return blob
}

// createTxConfig creates a transaction configuration with proper gas settings
func (s *StateQueuedSubmissionTestSuite) createTxConfig() *state.TxConfig {
	return state.NewTxConfig(
		state.WithGas(200_000),
		state.WithGasPrice(5000.0),
		state.WithMaxGasPrice(10000.0),
		state.WithTxPriority(1), // Medium priority
	)
}

// submitBlobViaJSONRPC submits a blob using the RPC client
func (s *StateQueuedSubmissionTestSuite) submitBlobViaJSONRPC(ctx context.Context, client *rpcclient.Client, blob *share.Blob, txConfig *state.TxConfig) (map[string]interface{}, error) {
	txResponse, err := client.State.SubmitPayForBlob(ctx, []*share.Blob{blob}, txConfig)
	if err != nil {
		return nil, err
	}

	response := map[string]interface{}{
		"result": map[string]interface{}{
			"txhash": txResponse.TxHash,
			"height": txResponse.Height,
			"code":   txResponse.Code,
		},
	}

	return response, nil
}

// isSuccessfulResponse checks if the JSON-RPC response indicates success
func (s *StateQueuedSubmissionTestSuite) isSuccessfulResponse(response map[string]interface{}) bool {
	if result, ok := response["result"].(map[string]interface{}); ok {
		if code, exists := result["code"]; exists {
			if codeFloat, ok := code.(float64); ok {
				return codeFloat == 0
			}
		}
		return true
	}
	return false
}
