package rpc

import (
	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

// Handler constructs a new RPC Handler from the given services.
func Handler(
	state state.Module,
	share share.Module,
	header header.Module,
	serv *rpc.Server,
	daser *das.DASer,
) {
	handler := rpc.NewHandler(state, share, header, daser)
	handler.RegisterEndpoints(serv)
}

func Server(cfg *Config) *rpc.Server {
	return rpc.NewServer(cfg.Address, cfg.Port)
}
