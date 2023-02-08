package swamp

import (
	"time"

	"github.com/celestiaorg/celestia-node/core"
)

// Components struct represents a set of pre-requisite attributes from the test scenario
type Components struct {
	*core.TestConfig
}

// DefaultComponents creates a celestia-app instance with a block time of around
// 100ms
func DefaultComponents() *Components {
	cfg := core.DefaultTestConfig()
	// timeout commits faster than this tend to be flakier
	cfg.Tendermint.Consensus.TimeoutCommit = 200 * time.Millisecond
	return &Components{
		cfg,
	}
}

// Option for modifying Swamp's Config.
type Option func(*Components)

// WithBlockTime sets a custom interval for block creation.
func WithBlockTime(t time.Duration) Option {
	return func(c *Components) {
		// for empty block
		c.Tendermint.Consensus.CreateEmptyBlocksInterval = t
		// for filled block
		c.Tendermint.Consensus.TimeoutCommit = t
		c.Tendermint.Consensus.SkipTimeoutCommit = false
	}
}
