package rpc

import (
	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/daser"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

// RegisterEndpoints registers the given services on the rpc.
func RegisterEndpoints(
	state state.Module,
	share share.Module,
	fraud fraud.Module,
	header header.Module,
	daser daser.Module,
	serv *rpc.Server,
) {
	serv.RegisterService("handler", state)
	serv.RegisterService("handler", share)
	serv.RegisterService("handler", fraud)
	serv.RegisterService("handler", header)
	serv.RegisterService("handler", daser)
}

func Server(cfg *Config) *rpc.Server {
	return rpc.NewServer(cfg.Address, cfg.Port)
}
