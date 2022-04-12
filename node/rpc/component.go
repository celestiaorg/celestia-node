package rpc

import (
	"go.uber.org/fx"
)

// RPC constructs a new RPC Server from the given Config.
// TODO @renaynay @Wondertan: this component is meant to be removed on implementation
//  of https://github.com/celestiaorg/celestia-node/pull/506.
func RPC(cfg Config) func(lc fx.Lifecycle) *Server {
	return func(lc fx.Lifecycle) *Server {
		serv := NewServer(cfg)
		lc.Append(fx.Hook{
			OnStart: serv.Start,
			OnStop:  serv.Stop,
		})
		return serv
	}
}
