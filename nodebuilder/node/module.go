package node

import (
	"github.com/cristalhq/jwt"
	"github.com/pyroscope-io/client/pyroscope"
	"go.uber.org/fx"
)

func ConstructModule(tp Type) fx.Option {
	return fx.Module(
		"node",
		fx.Provide(func(secret jwt.Signer) Module {
			return newModule(tp)
		}),
		fx.Invoke(func() {
			pyroscope.Start(pyroscope.Config{
				ApplicationName: "celestia.fullnode",
				// replace this with the address of pyroscope server
				ServerAddress: "http://localhost:4040",
				// you can disable logging by setting this to nil
				Logger: pyroscope.StandardLogger,
				// by default all profilers are enabled,
				// but you can select the ones you want to use:
				ProfileTypes: []pyroscope.ProfileType{
					pyroscope.ProfileCPU,
					pyroscope.ProfileAllocObjects,
					pyroscope.ProfileAllocSpace,
					pyroscope.ProfileInuseObjects,
					pyroscope.ProfileInuseSpace,
				},
			})
		}),
		fx.Provide(secret),
	)
}
