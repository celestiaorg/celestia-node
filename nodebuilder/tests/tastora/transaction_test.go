//go:build integration

package tastora

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/go-square/v3/share"
	sdk "github.com/cosmos/cosmos-sdk/types"

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

	// Initialize worker accounts sequentially to avoid race conditions
	s.T().Logf("Initializing worker accounts sequentially...")
	for i := 0; i < numParallelWorkers; i++ {
		// Create and submit blob to initialize worker account
		nodeBlobs, err := s.createTestBlob(ctx, client)
		require.NoError(s.T(), err)

		txConfig := state.NewTxConfig(
			state.WithGas(200_000),
			state.WithGasPrice(25000), // 0.025 utia per gas unit
		)

		_, err = client.Blob.Submit(ctx, nodeBlobs, txConfig)
		if err != nil {
			s.T().Logf("Worker account initialization %d failed: %v", i+1, err)
		} else {
			s.T().Logf("Worker account %d initialized successfully", i+1)
		}

		time.Sleep(100 * time.Millisecond)
	}

	s.T().Logf("Worker account initialization completed. Adding additional funding...")

	// Worker accounts are funded from the main node account
	// Ensure sufficient funds for multiple transaction rounds
	additionalFundingAmount := int64(50_000_000_000)
	fundingWallet := s.framework.getOrCreateFundingWallet(ctx)
	nodeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Get main node address and fund it
	nodeAddr, err := nodeClient.State.AccountAddress(ctx)
	require.NoError(s.T(), err, "failed to get node account address")
	nodeAccAddr := sdk.AccAddress(nodeAddr.Bytes())

	s.framework.fundWallet(ctx, fundingWallet, nodeAccAddr, additionalFundingAmount)
	s.T().Logf("Added %d utia additional funding to main node account", additionalFundingAmount)

	s.T().Logf("Additional funding completed. Starting parallel submission test...")

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

				// Create and submit blob
				nodeBlobs, err := s.createTestBlob(ctx, client)
				require.NoError(s.T(), err)

				txConfig := state.NewTxConfig(
					state.WithGas(200_000),
					state.WithGasPrice(25000), // 0.025 utia per gas unit
				)

				height, err := client.Blob.Submit(ctx, nodeBlobs, txConfig)

				mu.Lock()
				if err != nil {
					failureCount++
					s.T().Logf("Round %d, Worker %d: FAILED - %v", roundNum+1, workerNum+1, err)
				} else {
					successCount++
					s.T().Logf("Round %d, Worker %d: SUCCESS - height %d", roundNum+1, workerNum+1, height)
				}
				mu.Unlock()
			}(round, worker)
		}

		wg.Wait()
		s.T().Logf("Round %d completed", round+1)
		time.Sleep(500 * time.Millisecond)
	}

	// Verify all submissions succeeded
	totalSubmissions := numWorkers * numRounds
	s.Require().Equal(totalSubmissions, successCount+failureCount, "All submissions should be accounted for")

	successRate := float64(successCount) / float64(totalSubmissions)
	s.Require().Equal(successCount, totalSubmissions, "All parallel submissions should succeed (got %d/%d, %.2f%%)", successCount, totalSubmissions, successRate*100)
	s.Require().Equal(0, failureCount, "No parallel submissions should fail")

	s.T().Logf("Parallel submission test completed: %d/%d successful (%.2f%%), %d failed", successCount, totalSubmissions, successRate*100, failureCount)
}

// createTestBlob creates a test blob for parallel worker testing
func (s *TransactionTestSuite) createTestBlob(ctx context.Context, client *rpcclient.Client) ([]*nodeblob.Blob, error) {
	// Create namespace (10 bytes for V0)
	namespaceBytes := make([]byte, 10)
	for i := 0; i < 10; i++ {
		namespaceBytes[i] = byte(i)
	}

	namespace, err := share.NewV0Namespace(namespaceBytes)
	if err != nil {
		return nil, err
	}

	// Create test data
	data := []byte("parallel test blob data")

	// Use V0 blob (no signer required - worker accounts handle signing)
	shareBlob, err := share.NewV0Blob(namespace, data)
	if err != nil {
		return nil, err
	}

	// Convert to nodeblob.Blob
	nodeBlobs, err := nodeblob.ToNodeBlobs(shareBlob)
	if err != nil {
		return nil, err
	}

	return nodeBlobs, nil
}
