package rpc

import (
	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
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
	p2p p2p.Module,
	serv *rpc.Server,
) {
	serv.RegisterService("state", state)
	serv.RegisterService("share", share)
	serv.RegisterService("fraud", fraud)
	serv.RegisterService("header", header)
	serv.RegisterService("das", daser)
	serv.RegisterService("p2p", p2p)
}

func Server(cfg *Config) *rpc.Server {
	return rpc.NewServer(cfg.Address, cfg.Port)
}
