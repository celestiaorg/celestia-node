package node

import (
	"github.com/cristalhq/jwt/v5"
	"go.uber.org/fx"
)

func ConstructModule(tp Type) fx.Option {
	return fx.Module(
		"node",
		fx.Provide(func(signer jwt.Signer, verifier jwt.Verifier) Module {
			return newModule(tp, signer, verifier)
		}),
		fx.Provide(secret),
	)
}
