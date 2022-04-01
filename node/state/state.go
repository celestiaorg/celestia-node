package state

import (
	"github.com/celestiaorg/celestia-node/node/rpc"
	"github.com/celestiaorg/celestia-node/service/state"
)

func NewService(accessor state.Accessor, rpcServ *rpc.Server) *state.Service {
	serv := state.NewService(accessor)
	serv.RegisterEndpoints(rpcServ)
	return serv
}
