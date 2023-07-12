package swamp

import (
	"time"

	"github.com/celestiaorg/celestia-node/core"
)

// Config struct represents a set of pre-requisite attributes from the test scenario
type Config struct {
	*core.TestConfig
}

// DefaultConfig creates a celestia-app instance with a block time of around
// 100ms
func DefaultConfig() *Config {
	cfg := core.DefaultTestConfig()
	// timeout commit lower than this tend to be flakier
	cfg.Tendermint.Consensus.TimeoutCommit = 200 * time.Millisecond
	return &Config{
		cfg,
	}
}

// Option for modifying Swamp's Config.
type Option func(*Config)

// WithBlockTime sets a custom interval for block creation.
func WithBlockTime(t time.Duration) Option {
	return func(c *Config) {
		// for empty block
		c.Tendermint.Consensus.CreateEmptyBlocksInterval = t
		// for filled block
		c.Tendermint.Consensus.TimeoutCommit = t
		c.Tendermint.Consensus.SkipTimeoutCommit = false
	}
}
