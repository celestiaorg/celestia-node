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
	tnCfg.Consensus.TimeoutPrevote = 300 * time.Millisecond
	tnCfg.Consensus.TimeoutPrecommit = 300 * time.Millisecond
	tnCfg.Consensus.TimeoutPropose = 300 * time.Millisecond
	return &Components{
		App:     app,
		CoreCfg: tnCfg,
	}
}
