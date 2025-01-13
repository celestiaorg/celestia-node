package blobstream

import (
	"go.uber.org/fx"
)

func ConstructModule() fx.Option {
	return fx.Module("blobstream",
		fx.Provide(NewService),
		fx.Provide(func(serv *Service) Module {
			return serv
		}),
	)
}
