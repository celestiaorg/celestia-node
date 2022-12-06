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
	stateMod state.Module,
	shareMod share.Module,
	fraudMod fraud.Module,
	headerMod header.Module,
	daserMod das.Module,
	p2pMod p2p.Module,
	serv *rpc.Server,
) {
	serv.RegisterAuthedService("fraud", fraudMod, &fraud.API{})
	serv.RegisterAuthedService("das", daserMod, &das.API{})
	serv.RegisterAuthedService("header", headerMod, &header.API{})
	serv.RegisterAuthedService("state", stateMod, &state.API{})
	serv.RegisterAuthedService("share", shareMod, &share.API{})
	// @TODO(renaynay): add context to all p2p methods so we can activate auth
	serv.RegisterService("p2p", p2pMod)
}

func Server(cfg *Config) *rpc.Server {
	return rpc.NewServer(cfg.Address, cfg.Port)
}
