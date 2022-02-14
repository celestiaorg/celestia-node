package swamp

import (
	"time"

	"github.com/tendermint/tendermint/abci/types"
	tn "github.com/tendermint/tendermint/config"

	"github.com/celestiaorg/celestia-node/core"
)

// InfraComps struct represents a set of pre-requisite attributes from the test scenario
type InfraComps struct {
	App     types.Application
	CoreCfg *tn.Config
}

// DefaultInfraComps creates a KvStore with a block retention of 200
// In addition, the empty block interval is set to 200ms
func DefaultInfraComps() *InfraComps {
	app := core.CreateKvStore(200)
	tnCfg := tn.TestConfig()
	tnCfg.Consensus.CreateEmptyBlocksInterval = 200 * time.Millisecond
	return &InfraComps{
		App:     app,
		CoreCfg: tnCfg,
	}
}
