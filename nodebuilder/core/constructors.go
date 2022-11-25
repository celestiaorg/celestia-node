package core

import (
	"github.com/celestiaorg/celestia-node/core"
)

func Remote(cfg Config) (core.Client, error) {
	return core.NewRemote(cfg.IP, cfg.RPCPort)
}
