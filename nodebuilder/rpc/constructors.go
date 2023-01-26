package rpc

import (
	"github.com/cristalhq/jwt"

	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

// registerEndpoints registers the given services on the rpc.
func registerEndpoints(
	stateMod state.Module,
	shareMod share.Module,
	fraudMod fraud.Module,
	headerMod header.Module,
	daserMod das.Module,
	p2pMod p2p.Module,
	nodeMod node.Module,
	serv *rpc.Server,
) {
	serv.RegisterAuthedService("fraud", fraudMod, &fraud.API{})
	serv.RegisterAuthedService("das", daserMod, &das.API{})
	serv.RegisterAuthedService("header", headerMod, &header.API{})
	serv.RegisterAuthedService("state", stateMod, &state.API{})
	serv.RegisterAuthedService("share", shareMod, &share.API{})
	serv.RegisterAuthedService("p2p", p2pMod, &p2p.API{})
	serv.RegisterAuthedService("node", nodeMod, &node.API{})
}

func server(cfg *Config, auth jwt.Signer) *rpc.Server {
	return rpc.NewServer(cfg.Address, cfg.Port, auth)
}
