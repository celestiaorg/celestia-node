package rpc

import (
	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
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
	daser das.Module,
	serv *rpc.Server,
) {
	serv.RegisterService("state", state)
	serv.RegisterService("share", share)
	serv.RegisterService("fraud", fraud)
	serv.RegisterService("header", header)
	if daser != nil {
		serv.RegisterService("das", daser)
	}
}

func Server(cfg *Config) *rpc.Server {
	return rpc.NewServer(cfg.Address, cfg.Port)
}
