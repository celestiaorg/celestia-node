package swamp

import (
	"time"

	"github.com/tendermint/tendermint/abci/types"
	tn "github.com/tendermint/tendermint/config"

	"github.com/celestiaorg/celestia-node/core"
)

// InfraComponents struct represents a set of pre-requisite attributes from the test scenario
type InfraComponents struct {
	App     types.Application
	CoreCfg *tn.Config
}

// DefaultInfraComponents creates a KvStore with a block retention of 200
// In addition, the empty block interval is set to 200ms
func DefaultInfraComponents() *InfraComponents {
	app := core.CreateKvStore(200)
	tnCfg := tn.TestConfig()
	tnCfg.Consensus.CreateEmptyBlocksInterval = 200 * time.Millisecond
	return &InfraComponents{
		App:     app,
		CoreCfg: tnCfg,
	}
}
