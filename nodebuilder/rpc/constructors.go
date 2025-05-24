package rpc

import (
	"github.com/cristalhq/jwt/v5"

	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/blobstream"
	"github.com/celestiaorg/celestia-node/nodebuilder/da"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/modname"
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
	blobMod blob.Module,
	daMod da.Module,
	blobstreamMod blobstream.Module,
	serv *rpc.Server,
) {
	serv.RegisterService(modname.Fraud, fraudMod, &fraud.API{})
	serv.RegisterService(modname.DAS, daserMod, &das.API{})
	serv.RegisterService(modname.Header, headerMod, &header.API{})
	serv.RegisterService(modname.State, stateMod, &state.API{})
	serv.RegisterService(modname.Share, shareMod, &share.API{})
	serv.RegisterService(modname.P2P, p2pMod, &p2p.API{})
	serv.RegisterService(modname.Node, nodeMod, &node.API{})
	serv.RegisterService(modname.Blob, blobMod, &blob.API{})
	serv.RegisterService(modname.DA, daMod, &da.API{})
	serv.RegisterService(modname.Blobstream, blobstreamMod, &blobstream.API{})
}

func server(cfg *Config, signer jwt.Signer, verifier jwt.Verifier) *rpc.Server {
	return rpc.NewServer(cfg.Address, cfg.Port, cfg.SkipAuth, signer, verifier)
}
