package rpc

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/rpc"
	"github.com/celestiaorg/celestia-node/service/share"
	"github.com/celestiaorg/celestia-node/service/state"
)

// ServerComponent constructs a new RPC Server from the given Config.
// TODO @renaynay @Wondertan: this component is meant to be removed on implementation
//  of https://github.com/celestiaorg/celestia-node/pull/506.
func ServerComponent(cfg rpc.Config) func(lc fx.Lifecycle) *rpc.Server {
	return func(lc fx.Lifecycle) *rpc.Server {
		serv := rpc.NewServer(cfg)
		lc.Append(fx.Hook{
			OnStart: serv.Start,
			OnStop:  serv.Stop,
		})
		return serv
	}
}

func HandlerComponents(
	state *state.Service,
	share *share.Service,
	header *header.Service,
	serv *rpc.Server,
) {
	handler := rpc.NewHandler(state, share, header)
	handler.RegisterEndpoints(serv)
}
