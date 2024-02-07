package da

import (
	"go.uber.org/fx"
)

func ConstructModule() fx.Option {
	return fx.Module("da",
		fx.Provide(NewService),
		fx.Provide(func(serv *Service) Module {
			return serv
		}),
	)
}
