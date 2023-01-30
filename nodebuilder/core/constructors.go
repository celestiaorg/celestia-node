package core

import (
	"github.com/celestiaorg/celestia-node/core"
)

func remote(cfg Config) (core.Client, error) {
	return core.NewRemote(cfg.IP, cfg.RPCPort)
}
