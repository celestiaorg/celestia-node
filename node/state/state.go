package state

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/service/state"
)

func NewService(lc fx.Lifecycle, accessor state.Accessor, fService fraud.Service) *state.Service {
	serv := state.NewService(accessor, fService)
	lc.Append(fx.Hook{
		OnStart: serv.Start,
		OnStop:  serv.Stop,
	})
	return serv
}
