////go:build integration

package tastora

import (
	"context"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/go-square/v3/share"

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

	// spawn routine that keeps filling blocks

	fromAddr := s.framework.fundingWallet.Address
	wal, err := s.framework.GetBridgeNodes()[0].GetWallet()
	require.NoError(s.T(), err)
	toAddr := wal.Address

	bankSend := banktypes.NewMsgSend(fromAddr, toAddr, sdk.NewCoins(sdk.NewCoin("utia", sdkmath.NewInt(1))))
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, _ = s.framework.celestia.BroadcastMessages(ctx, s.framework.fundingWallet, bankSend)
			}
		}
	}()
}

func (s *TransactionTestSuite) TearDownSuite() {
	if s.framework != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.framework.Stop(ctx)
	}
}

func (s *TransactionTestSuite) TestSubmitParallelTxs() {
	defer s.TearDownSuite()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	_, err := client.Header.WaitForHeight(ctx, 2)
	require.NoError(s.T(), err)

	var (
		numRounds    = 5
		failureCount atomic.Int32
		wg           sync.WaitGroup
	)

	for round := 0; round < numRounds; round++ {
		s.T().Logf("Starting round %d with %d parallel workers", round+1, numParallelWorkers)
		for worker := 0; worker < numParallelWorkers; worker++ {
			wg.Add(1)
			go func(roundNum, workerNum int) {
				defer wg.Done()

				// Create and submit blob
				nodeBlobs, err := s.createTestBlob()
				require.NoError(s.T(), err)

				txConfig := state.NewTxConfig(
					state.WithGas(200_000),
					state.WithGasPrice(25000), // 0.025 utia per gas unit
				)

				height, err := client.Blob.Submit(ctx, nodeBlobs, txConfig)
				if err != nil {
					failureCount.Add(1)
					s.T().Logf("Round %d, Worker %d: FAILED - %v", roundNum+1, workerNum+1, err)
					return
				}
				s.T().Logf("Round %d, Worker %d: SUCCESS - height %d", roundNum+1, workerNum+1, height)
			}(round, worker)
		}

		wg.Wait()
		s.T().Logf("Round %d completed", round+1)
	}

	// TODO @renaynay: delete this when u have figured out bug
	_, err = client.Header.WaitForHeight(ctx, 500)
	require.NoError(s.T(), err)

	// Verify all submissions succeeded
	s.Require().Equal(0, failureCount.Load(), "No parallel submissions should fail")
	s.T().Logf("Parallel submission test completed: %d failed", failureCount.Load())
}

// createTestBlob creates a test blob for parallel worker testing
func (s *TransactionTestSuite) createTestBlob() ([]*nodeblob.Blob, error) {
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
