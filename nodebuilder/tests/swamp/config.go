package swamp

import (
	"time"

	"github.com/celestiaorg/celestia-app/test/util/malicious"
	"github.com/celestiaorg/celestia-app/test/util/testnode"

	"github.com/celestiaorg/celestia-node/core"
)

// DefaultConfig creates a celestia-app instance with a block time of around
// 100ms
func DefaultConfig() *testnode.Config {
	cfg := core.DefaultTestConfig()
	// timeout commit lower than this tend to be flakier
	cfg.TmConfig.Consensus.TimeoutCommit = 200 * time.Millisecond
	return cfg
}

// Option for modifying Swamp's Config.
type Option func(*testnode.Config)

// WithBlockTime sets a custom interval for block creation.
func WithBlockTime(t time.Duration) Option {
	return func(c *testnode.Config) {
		// for empty block
		c.TmConfig.Consensus.CreateEmptyBlocksInterval = t
		// for filled block
		c.TmConfig.Consensus.TimeoutCommit = t
		c.TmConfig.Consensus.SkipTimeoutCommit = false
	}
}

// WithOutOfOrderShares sets a height after which app will start producing invalid blocks.
func WithOutOfOrderShares(startHeight int64) Option {
	return func(config *testnode.Config) {
		cfg := malicious.OutOfOrderNamespaceConfig(startHeight)
		behaviour := cfg.AppOptions.Get(malicious.BehaviorConfigKey)

		config.AppCreator = cfg.AppCreator
		config.AppOptions.Set(malicious.BehaviorConfigKey, behaviour)
	}
}
