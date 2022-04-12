package rpc

import (
	"go.uber.org/fx"
)

func RPC(cfg Config) func(lc fx.Lifecycle) *Server {
	return func(lc fx.Lifecycle) *Server {
		// TODO @renaynay @Wondertan: not providing any custom config
		//  functionality here as this component is meant to be removed on
		//  implementation of https://github.com/celestiaorg/celestia-node/pull/506.
		serv := NewServer(cfg)
		lc.Append(fx.Hook{
			OnStart: serv.Start,
			OnStop:  serv.Stop,
		})
		return serv
	}
}
