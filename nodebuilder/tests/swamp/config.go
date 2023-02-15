package swamp

import (
	"time"

	logging "github.com/ipfs/go-log/v2"

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

// WithLogLevel overrides basic log levels for test purposes
func WithLogLevel(components []string, levels []string) Option {
	return func(_ *Components) {
		if len(components) != len(levels) && len(levels) != 1 {
			panic("inconsistent amount of components and levels")
		}

		level := levels[0]
		for i, c := range components {
			if len(levels) == len(components) {
				level = levels[i]
			}
			_ = logging.SetLogLevel(c, level)
		}
	}
}
