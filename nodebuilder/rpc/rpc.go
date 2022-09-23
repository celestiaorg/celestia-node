package rpc

import (
	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/celestiaorg/celestia-node/service/rpc"
	"github.com/celestiaorg/celestia-node/service/share"
)

// Handler constructs a new RPC Handler from the given services.
func Handler(
	state state.Service,
	share *share.Service,
	header header.Service,
	serv *rpc.Server,
	daser *das.DASer,
) {
	handler := rpc.NewHandler(state, share, header, daser)
	handler.RegisterEndpoints(serv)
	handler.RegisterMiddleware(serv)
}
