package swamp

import (
	"time"

	tn "github.com/tendermint/tendermint/config"
)

// Components struct represents a set of pre-requisite attributes from the test scenario
type Components struct {
	CoreCfg *tn.Config
}

// DefaultComponents creates a KvStore with a block retention of 200
// In addition, the empty block interval is set to 200ms
func DefaultComponents() *Components {
	tnCfg := tn.ResetTestRoot("swamp_tests")
	tnCfg.Consensus.CreateEmptyBlocksInterval = 100 * time.Millisecond
	return &Components{
		CoreCfg: tnCfg,
	}
}

// Option for modifying Swamp's Config.
type Option func(*Components)

// WithBlockTime sets a custom interval for block creation.
func WithBlockTime(t time.Duration) Option {
	return func(c *Components) {
		// for empty block
		c.CoreCfg.Consensus.CreateEmptyBlocksInterval = t
		// for filled block
		c.CoreCfg.Consensus.TimeoutCommit = t
		c.CoreCfg.Consensus.SkipTimeoutCommit = true
	}
}
