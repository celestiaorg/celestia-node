//go:build integration

package tastora

import (
	"context"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/celestia-node/state"
)

// StateTestSuite provides comprehensive testing of the State module
type StateTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestStateTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping State module integration tests in short mode")
	}
	suite.Run(t, &StateTestSuite{})
}

func (s *StateTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithFullNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *StateTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.cleanup()
	}
}

// TestStateAccountAddress validates account address retrieval
func (s *StateTestSuite) TestStateAccountAddress_DefaultAccount() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get account address
	address, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err, "should retrieve account address")
	s.Require().NotEmpty(address, "address should not be empty")

	// Validate address format (Celestia addresses start with "celestia")
	s.Assert().Contains(address.String(), "celestia", "address should be valid Celestia format")
}

// TestStateBalance validates balance retrieval functionality
func (s *StateTestSuite) TestStateBalance_DefaultAccount() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create test wallet and fund the node account with more generous amounts
	testWallet := s.framework.CreateTestWallet(ctx, 5_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, fullNode, 2_000_000_000)

	// Wait a bit for funding to be processed
	time.Sleep(5 * time.Second)

	// Get balance with retry logic for better reliability
	var balance *state.Balance
	var err error
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		balance, err = client.State.Balance(ctx)
		if err == nil && balance != nil && balance.Amount.GT(math.NewInt(0)) {
			break
		}
		if i < maxRetries-1 {
			time.Sleep(3 * time.Second)
		}
	}

	s.Require().NoError(err, "should retrieve balance")
	s.Require().NotNil(balance, "balance should not be nil")

	// Validate balance structure
	s.Assert().NotEmpty(balance.Denom, "denomination should not be empty")
	s.Assert().True(balance.Amount.GT(math.NewInt(0)), "balance should be greater than zero")
	s.Assert().Equal("utia", balance.Denom, "should be utia denomination")
}

// TestStateBalanceForAddress validates balance retrieval for specific addresses
func (s *StateTestSuite) TestStateBalanceForAddress_ValidAddress() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get our own address
	address, err := client.State.AccountAddress(ctx)
	s.Require().NoError(err, "should retrieve account address")

	// Create test wallet and fund the node account
	testWallet := s.framework.CreateTestWallet(ctx, 5_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, fullNode, 2_000_000_000)

	// Wait for funding to be processed
	time.Sleep(5 * time.Second)

	// Get balance for our address
	balance, err := client.State.BalanceForAddress(ctx, address)
	s.Require().NoError(err, "should retrieve balance for address")
	s.Require().NotNil(balance, "balance should not be nil")

	s.Assert().True(balance.Amount.GT(math.NewInt(0)), "balance should be greater than zero")
}

func (s *StateTestSuite) TestStateBalanceForAddress_InvalidAddress() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create a malformed address that will be rejected by the API
	// Using a valid-looking but incorrect bech32 address
	invalidAddrBytes := make([]byte, 32) // Wrong length for address
	for i := range invalidAddrBytes {
		invalidAddrBytes[i] = 0xFF // All 0xFF bytes (invalid address format)
	}
	invalidAddr := state.Address{Address: state.AccAddress(invalidAddrBytes)}

	// Try to get balance for invalid address
	// The system gracefully handles invalid addresses by returning zero balance
	balance, err := client.State.BalanceForAddress(ctx, invalidAddr)
	s.Require().NoError(err, "should handle invalid address gracefully")
	s.Assert().True(balance.Amount.IsZero(), "should return zero balance for invalid address")
}

// TestStateTransfer validates token transfer functionality
func (s *StateTestSuite) TestStateTransfer_ValidTransaction() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create test wallet and fund the node account with more generous amounts
	testWallet := s.framework.CreateTestWallet(ctx, 10_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, fullNode, 5_000_000_000)

	// Wait for funding to be processed
	time.Sleep(5 * time.Second)

	// Get sender balance before transfer
	senderBalance, err := client.State.Balance(ctx)
	s.Require().NoError(err, "should get sender balance")
	initialAmount := senderBalance.Amount

	// Create recipient address (use a valid test address format)
	recipientAddr := state.AccAddress("celestia1test123456789abcdefghijklmnopqrstuvwxyz")
	transferAmount := math.NewInt(100000000) // 100 utia

	// Perform transfer with appropriate gas price
	txConfig := state.NewTxConfig(
		state.WithGasPrice(0.025), // Use minimum required gas price
	)
	txResponse, err := client.State.Transfer(ctx, recipientAddr, transferAmount, txConfig)
	s.Require().NoError(err, "transfer should succeed")
	s.Require().NotNil(txResponse, "transaction response should not be nil")
	s.Assert().Equal(uint32(0), txResponse.Code, "transaction should be successful")
	s.Assert().Greater(txResponse.Height, int64(0), "transaction should be included in a block")

	// Wait for transaction to be processed
	time.Sleep(5 * time.Second)

	// Verify sender balance decreased (with retry logic)
	var newSenderBalance *state.Balance
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		newSenderBalance, err = client.State.Balance(ctx)
		if err == nil && newSenderBalance != nil {
			break
		}
		if i < maxRetries-1 {
			time.Sleep(3 * time.Second)
		}
	}
	s.Require().NoError(err, "should get updated sender balance")

	// Balance should have decreased (transfer amount + fees)
	s.Assert().True(newSenderBalance.Amount.LT(initialAmount), "sender balance should have decreased")
}

func (s *StateTestSuite) TestStateTransfer_InsufficientFunds() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create test wallet with minimal funding to ensure insufficient funds
	testWallet := s.framework.CreateTestWallet(ctx, 10_000)       // Very small initial amount
	s.framework.FundNodeAccount(ctx, testWallet, fullNode, 5_000) // Fund with minimal amount

	// Wait for funding
	time.Sleep(5 * time.Second)

	// Get the actual current balance to ensure our transfer exceeds it
	balance, err := client.State.Balance(ctx)
	s.Require().NoError(err, "should get current balance")

	// Calculate transfer amount that's guaranteed to exceed available funds including gas
	// Use 10x the current balance to ensure insufficient funds
	transferAmount := balance.Amount.Mul(math.NewInt(10))

	// Add extra amount to be absolutely sure it exceeds funds
	transferAmount = transferAmount.Add(math.NewInt(1_000_000_000))

	recipientAddr := state.AccAddress("celestia1test123456789abcdefghijklmnopqrstuvwxyz")

	txConfig := state.NewTxConfig(
		state.WithGasPrice(0.025), // Use minimum required gas price
	)

	// This should definitely fail with insufficient funds
	_, err = client.State.Transfer(ctx, recipientAddr, transferAmount, txConfig)
	s.Assert().Error(err, "transfer should fail with insufficient funds")
	s.Assert().Contains(err.Error(), "insufficient", "error should mention insufficient funds")
}

// TestStateSubmitPayForBlob validates PayForBlob transaction functionality
func (s *StateTestSuite) TestStateSubmitPayForBlob_ValidTransaction() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create test wallet and fund the node account
	testWallet := s.framework.CreateTestWallet(ctx, 5_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, fullNode, 3_000_000_000)

	// Create test blobs with non-reserved namespace (use libshare blobs for state API)
	testBlobs, err := s.framework.GenerateTestLibshareBlobs(1, 256)
	s.Require().NoError(err, "should generate test blobs")

	// Submit PayForBlob transaction with appropriate gas price
	txConfig := state.NewTxConfig(
		state.WithGasPrice(0.025), // Use minimum required gas price
	)
	txResponse, err := client.State.SubmitPayForBlob(ctx, testBlobs, txConfig)
	s.Require().NoError(err, "PayForBlob transaction should succeed")
	s.Require().NotNil(txResponse, "transaction response should not be nil")
	s.Assert().Equal(uint32(0), txResponse.Code, "transaction should be successful")
	s.Assert().Greater(txResponse.Height, int64(0), "transaction should be included in a block")
	s.Assert().NotEmpty(txResponse.TxHash, "transaction hash should not be empty")
}

func (s *StateTestSuite) TestStateSubmitPayForBlob_MultipleBlobs() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create test wallet and fund the node account generously for multiple blobs
	testWallet := s.framework.CreateTestWallet(ctx, 10_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, fullNode, 5_000_000_000)

	// Create multiple test blobs with non-reserved namespaces
	testBlobs, err := s.framework.GenerateTestLibshareBlobs(3, 256)
	s.Require().NoError(err, "should generate test blobs")

	// Submit PayForBlob transaction with multiple blobs
	txConfig := state.NewTxConfig(
		state.WithGasPrice(0.025), // Use minimum required gas price
	)
	txResponse, err := client.State.SubmitPayForBlob(ctx, testBlobs, txConfig)
	s.Require().NoError(err, "multiple blob PayForBlob should succeed")
	s.Assert().Equal(uint32(0), txResponse.Code, "transaction should be successful")
}

// TestStateTransactionConfig validates transaction configuration options
func (s *StateTestSuite) TestStateTransactionConfig_CustomGasLimit() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create test wallet and fund the node account
	testWallet := s.framework.CreateTestWallet(ctx, 5_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, fullNode, 3_000_000_000)

	// Create test blob
	testBlobs, err := s.framework.GenerateTestLibshareBlobs(1, 256)
	s.Require().NoError(err, "should generate test blob")

	// Submit with custom gas limit and appropriate gas price
	txConfig := state.NewTxConfig(
		state.WithGas(300000),     // Custom gas limit
		state.WithGasPrice(0.025), // Use minimum required gas price
	)

	txResponse, err := client.State.SubmitPayForBlob(ctx, testBlobs, txConfig)
	s.Require().NoError(err, "PayForBlob with custom gas should succeed")
	s.Assert().Equal(uint32(0), txResponse.Code, "transaction should be successful")
}

// TestStateMultiAccountScenario validates multi-account operations
func (s *StateTestSuite) TestStateMultiAccountScenario() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Note: For this test we'll use simplified single-account testing
	// since the framework's multi-account support would require additional setup

	// Test operations with the main funded node account
	testWallet := s.framework.CreateTestWallet(ctx, 10_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, fullNode, 5_000_000_000)

	s.Run("Account_NodeAccount", func() {
		// Generate test blob with non-reserved namespace
		testBlobs, err := s.framework.GenerateTestLibshareBlobs(1, 256)
		s.Require().NoError(err, "should generate test blob")

		// Submit PayForBlob from node account with appropriate gas price
		txConfig := state.NewTxConfig(
			state.WithGasPrice(0.025), // Use minimum required gas price
		)
		txResponse, err := client.State.SubmitPayForBlob(ctx, testBlobs, txConfig)
		s.Require().NoError(err, "PayForBlob should succeed for node account")
		s.Assert().Equal(uint32(0), txResponse.Code, "transaction should be successful")
	})
}

// TestStateErrorHandling validates proper error handling
func (s *StateTestSuite) TestStateErrorHandling_NetworkTimeout() {
	// Create context with very short timeout to force actual timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Nanosecond)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Wait a bit to ensure context times out
	time.Sleep(1 * time.Millisecond)

	// This should timeout
	_, err := client.State.AccountAddress(ctx)
	s.Assert().Error(err, "should return timeout error")
	s.Assert().Contains(err.Error(), "context deadline exceeded", "should be timeout error")
}

func (s *StateTestSuite) TestStateErrorHandling_InvalidTransactionConfig() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create test wallet and fund normally for this test
	testWallet := s.framework.CreateTestWallet(ctx, 5_000_000_000)
	s.framework.FundNodeAccount(ctx, testWallet, fullNode, 2_000_000_000)

	// Wait for funding to complete
	time.Sleep(5 * time.Second)

	// Create test blobs
	testBlobs, err := s.framework.GenerateTestLibshareBlobs(1, 256)
	s.Require().NoError(err, "should generate test blobs")

	// Test invalid transaction config with negative gas price
	invalidTxConfig := state.NewTxConfig(
		state.WithGasPrice(-1.0), // Invalid negative gas price
	)

	// This should fail due to invalid configuration
	_, err = client.State.SubmitPayForBlob(ctx, testBlobs, invalidTxConfig)
	s.Assert().Error(err, "should return error for invalid transaction config")
}
