package swamp

import (
	"time"

	"github.com/tendermint/tendermint/abci/types"
	tn "github.com/tendermint/tendermint/config"

	"github.com/celestiaorg/celestia-node/core"
)

// Components struct represents a set of pre-requisite attributes from the test scenario
type Components struct {
	App     types.Application
	CoreCfg *tn.Config
}

// DefaultComponents creates a KvStore with a block retention of 200
// In addition, the empty block interval is set to 200ms
func DefaultComponents() *Components {
	app := core.CreateKVStore(200)
	tnCfg := tn.TestConfig()
	tnCfg.Consensus.CreateEmptyBlocksInterval = 200 * time.Millisecond
	return &Components{
		App:     app,
		CoreCfg: tnCfg,
	}
}

// Option for modifying Swamp's Config.
type Option func(*Components)

// WithBlockInterval sets a custom interval for block creation.
func WithBlockInterval(interval time.Duration) Option {
	return func(c *Components) {
		c.CoreCfg.Consensus.CreateEmptyBlocksInterval = interval
	}
}
