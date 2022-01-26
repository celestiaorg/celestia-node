package swamp

import (
	"time"

	"github.com/tendermint/tendermint/abci/types"
	tn "github.com/tendermint/tendermint/config"

	"github.com/celestiaorg/celestia-node/core"
)

// Config struct represents a set of pre-requisite attributes from the test scenario
type Config struct {
	App     types.Application
	CoreCfg *tn.Config
}

// DefaultConfig creates a KvStore with a block retention of 200
// In addition, the empty block interval is set to 100ms
func DefaultConfig() *Config {
	app := core.CreateKvStore(200)
	tnCfg := tn.TestConfig()
	tnCfg.Consensus.CreateEmptyBlocksInterval = 100 * time.Millisecond
	return &Config{
		App:     app,
		CoreCfg: tnCfg,
	}
}
