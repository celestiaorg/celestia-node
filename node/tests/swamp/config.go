package swamp

import (
	"time"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/tendermint/tendermint/abci/types"
	tn "github.com/tendermint/tendermint/config"
)

type Config struct {
	App     types.Application
	CoreCfg *tn.Config
}

func DefaultConfig() *Config {
	app := core.CreateKvStore(200)
	tnCfg := tn.TestConfig()
	tnCfg.Consensus.CreateEmptyBlocksInterval = 100 * time.Millisecond
	return &Config{
		App:     app,
		CoreCfg: tnCfg,
	}
}
