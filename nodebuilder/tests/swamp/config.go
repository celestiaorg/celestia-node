package swamp

import (
	"time"

	"github.com/celestiaorg/celestia-app/test/util/testnode"
	"github.com/celestiaorg/celestia-node/core"
)

// DefaultConfig creates a celestia-app instance with a block time of around
// 100ms
func DefaultConfig() *testnode.Config {
	cfg := core.DefaultTestConfig()
	// target height duration lower than this tend to be flakier
	cfg.TmConfig.Consensus.TargetHeightDuration = 200 * time.Millisecond
	return cfg
}

// Option for modifying Swamp's Config.
type Option func(*testnode.Config)

// WithBlockTime sets a custom interval for block creation.
func WithBlockTime(t time.Duration) Option {
	return func(c *testnode.Config) {
		// for filled block
		c.TmConfig.Consensus.TargetHeightDuration = t
	}
}
