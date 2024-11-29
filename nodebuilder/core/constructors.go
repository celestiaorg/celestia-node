package core

import (
	"github.com/celestiaorg/celestia-node/core"
)

func remote(cfg Config) *core.Client {
	return core.NewClient(cfg.IP, cfg.Port)
}
