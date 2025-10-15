//go:build integration

package tastora

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/go-square/v3/share"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/state"
)

const numParallelWorkers = 8

type TransactionTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestTransactionTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Transaction integration tests in short mode")
	}
	suite.Run(t, &TransactionTestSuite{})
}

func (s *TransactionTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithTxWorkerAccounts(numParallelWorkers))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *TransactionTestSuite) TearDownSuite() {
	if s.framework != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.framework.Stop(ctx)
	}
}

func (s *TransactionTestSuite) TestSubmitParallelTxs() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	_, err := client.Header.WaitForHeight(ctx, 1)
	require.NoError(s.T(), err)

	// This test is designed to verify that the --tx.worker.accounts feature works correctly.
	// The node should create and manage worker accounts for parallel transaction submission.
	// If this test fails with "parallel-worker-X not found" errors, it indicates that
	// the worker account creation logic is broken in the celestia-node implementation.

	const numWorkers = numParallelWorkers
	const numRounds = 2
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	failureCount := 0

	for round := 0; round < numRounds; round++ {
		s.T().Logf("Starting round %d with %d parallel workers", round+1, numParallelWorkers)
		for worker := 0; worker < numParallelWorkers; worker++ {
			wg.Add(1)
			go func(roundNum, workerNum int) {
				defer wg.Done()

				// Create a unique blob for this worker/round
				nodeBlobs, err := s.createTestBlob(ctx, client)
				require.NoError(s.T(), err)

				txConfig := state.NewTxConfig(
					state.WithGas(200_000),
					state.WithGasPrice(25000), // 0.025 utia per gas unit (minimum required)
				)

				// Submit the blob
				height, err := client.Blob.Submit(ctx, nodeBlobs, txConfig)

				mu.Lock()
				if err != nil {
					failureCount++
					// Log detailed error information to help diagnose the worker account issue
					s.T().Logf("Round %d, Worker %d: FAILED - %v", roundNum+1, workerNum+1, err)
					if strings.Contains(err.Error(), "parallel-worker") {
						s.T().Logf("WORKER ACCOUNT ERROR: %v", err)
					}
				} else {
					successCount++
					s.T().Logf("Round %d, Worker %d: SUCCESS - height %d", roundNum+1, workerNum+1, height)
				}
				mu.Unlock()
			}(round, worker)
		}

		// Wait for all workers in this round to complete
		wg.Wait()
		s.T().Logf("Round %d completed", round+1)

		// Small delay between rounds
		time.Sleep(500 * time.Millisecond)
	}

	// Verify results
	totalSubmissions := numWorkers * numRounds
	s.Require().Equal(totalSubmissions, successCount+failureCount, "All submissions should be accounted for")
	s.Require().Equal(totalSubmissions, successCount, "All parallel submissions should succeed")
	s.Require().Equal(0, failureCount, "No parallel submissions should fail")

	s.T().Logf("Parallel submission test completed: %d/%d successful, %d failed", successCount, totalSubmissions, failureCount)
}

// createTestBlob creates a test blob with unique data
func (s *TransactionTestSuite) createTestBlob(ctx context.Context, client *rpcclient.Client) ([]*nodeblob.Blob, error) {
	// Create namespace with proper format (10 bytes for version 0)
	namespaceBytes := make([]byte, 10)
	// Fill with test data
	for i := 0; i < 10; i++ {
		namespaceBytes[i] = byte(i)
	}

	// Create namespace from bytes
	namespace, err := share.NewV0Namespace(namespaceBytes)
	if err != nil {
		return nil, err
	}

	// Create test data
	data := []byte("parallel test blob data")

	// Get the actual node address as the signer
	nodeAddr, err := client.State.AccountAddress(ctx)
	if err != nil {
		return nil, err
	}

	shareBlob, err := share.NewV1Blob(namespace, data, nodeAddr.Bytes())
	if err != nil {
		return nil, err
	}

	// Convert to nodeblob.Blob using ToNodeBlobs
	nodeBlobs, err := nodeblob.ToNodeBlobs(shareBlob)
	if err != nil {
		return nil, err
	}

	return nodeBlobs, nil
}
