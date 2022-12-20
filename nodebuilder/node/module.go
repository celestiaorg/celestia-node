package node

import (
	"github.com/cristalhq/jwt"
	"go.uber.org/fx"
)

func ConstructModule(tp Type) fx.Option {
	return fx.Module(
		"node",
		fx.Provide(func(secret jwt.Signer) Module {
			return newModule(tp)
		}),
		fx.Provide(secret),
	)
}
