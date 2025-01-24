package core

import (
	"testing"
	"time"

	tmrand "github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/celestia-app/v3/test/util/genesis"
	"github.com/celestiaorg/celestia-app/v3/test/util/testnode"
)

const chainID = "private"

// DefaultTestConfig returns the default testing configuration for Tendermint + Celestia App tandem.
//
// It fetches free ports from OS and sets them into configs, s.t.
// user can make use of them(unlike 0 port) and allowing to run
// multiple tests nodes in parallel.
//
// Additionally, it instructs Tendermint + Celestia App tandem to setup 10 funded accounts.
func DefaultTestConfig() *testnode.Config {
	genesis := genesis.NewDefaultGenesis().
		WithChainID(chainID).
		WithValidators(genesis.NewDefaultValidator(testnode.DefaultValidatorAccountName)).
		WithConsensusParams(testnode.DefaultConsensusParams())

	tmConfig := testnode.DefaultTendermintConfig()
	tmConfig.Consensus.TimeoutCommit = time.Millisecond * 200

	return testnode.DefaultConfig().
		WithChainID(chainID).
		WithFundedAccounts(generateRandomAccounts(10)...). // 10 usually is enough for testing
		WithGenesis(genesis).
		WithTendermintConfig(tmConfig).
		WithSuppressLogs(true)
}

// StartTestNode simply starts Tendermint and Celestia App tandem with default testing
// configuration.
func StartTestNode(t *testing.T) testnode.Context {
	return StartTestNodeWithConfig(t, DefaultTestConfig())
}

// StartTestNodeWithConfig starts Tendermint and Celestia App tandem with custom configuration.
func StartTestNodeWithConfig(t *testing.T, cfg *testnode.Config) testnode.Context {
	cctx, _, _ := testnode.NewNetwork(t, cfg)
	// we want to test over remote http client,
	// so we are as close to the real environment as possible,
	// however, it might be useful to use a local tendermint client
	// if you need to debug something inside it
	return cctx
}

// generateRandomAccounts generates n random account names.
func generateRandomAccounts(n int) []string {
	accounts := make([]string, n)
	for i := range accounts {
		accounts[i] = tmrand.Str(9)
	}
	return accounts
}
