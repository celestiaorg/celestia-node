package rpc

import (
	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

// RegisterEndpoints registers the given services on the rpc.
func RegisterEndpoints(
	state state.Module,
	share share.Module,
	header header.Module,
	serv *rpc.Server,
	daser *das.DASer,
) {
	serv.RegisterService("state", state)
	serv.RegisterService("share", share)
	serv.RegisterService("header", header)
	serv.RegisterService("daser", daser)
}

func Server(cfg *Config) *rpc.Server {
	return rpc.NewServer(cfg.Address, cfg.Port)
}
