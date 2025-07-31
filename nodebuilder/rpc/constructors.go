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
	serv.RegisterService("fraud", fraudMod, &fraud.API{})
	serv.RegisterService("das", daserMod, &das.API{})
	serv.RegisterService("header", headerMod, &header.API{})
	serv.RegisterService("state", stateMod, &state.API{})
	serv.RegisterService("share", shareMod, &share.API{})
	serv.RegisterService("p2p", p2pMod, &p2p.API{})
	serv.RegisterService("node", nodeMod, &node.API{})
	serv.RegisterService("blob", blobMod, &blob.API{})
	serv.RegisterService("da", daMod, &da.API{})
	serv.RegisterService("blobstream", blobstreamMod, &blobstream.API{})
}

func server(cfg *Config, signer jwt.Signer, verifier jwt.Verifier) *rpc.Server {
	return rpc.NewServer(cfg.Address, cfg.Port, cfg.SkipAuth, rpc.CORSConfig{
		Enabled:        cfg.CORS.Enabled,
		AllowedOrigins: cfg.CORS.AllowedOrigins,
		AllowedMethods: cfg.CORS.AllowedMethods,
		AllowedHeaders: cfg.CORS.AllowedHeaders,
	}, signer, verifier)
}
