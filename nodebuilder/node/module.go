package node

import (
	"go.uber.org/fx"
)

func ConstructModule(tp Type) fx.Option {
	return fx.Module(
		"node",
		fx.Provide(func() Module {
			return newAdmin(tp)
		}),
	)
}
