//go:build integration

package tastora

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Create and submit blob
	nodeBlobs, err := s.createTestBlob()
	require.NoError(s.T(), err)

	// TODO @renaynay: this  is a very ugly hack that
	//  waits for tx client to be ready
	for i := 0; i < 3; i++ {
		height, err := client.Blob.Submit(ctx, nodeBlobs, state.NewTxConfig())
		if err == nil && height > 0 {
			break
		}
	}

	var (
		numRounds    = 3
		failureCount atomic.Int32
		wg           sync.WaitGroup
	)
	for round := 0; round < numRounds; round++ {
		s.T().Logf("Starting round %d with %d parallel workers", round+1, numParallelWorkers)
		for worker := 0; worker < numParallelWorkers; worker++ {
			wg.Add(1)
			go func(roundNum, workerNum int) {
				defer wg.Done()

				// Use a shorter timeout per submission to fail fast
				submitCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
				defer cancel()

				// Retry submission once for transient RPC failures
				var height uint64
				var err error
				for attempt := 1; attempt <= 2; attempt++ {
					height, err = client.Blob.Submit(submitCtx, nodeBlobs, state.NewTxConfig())
					if err == nil && height > 0 {
						break
					}
					if attempt < 2 {
						time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
					}
				}

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

		// Add shorter delay between rounds to allow bridge node to recover
		if round < numRounds-1 {
			s.T().Logf("Waiting 2 seconds before starting next round...")
			time.Sleep(2 * time.Second)
		}
	}

	// Verify all submissions succeeded
	s.Require().Equal(int32(0), failureCount.Load(), "No parallel submissions should fail")
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
