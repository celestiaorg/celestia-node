package blobstream

import (
	"go.uber.org/fx"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
)

func ConstructModule() fx.Option {
	return fx.Module("blobstream",
		fx.Provide(func(store libhead.Store[*header.ExtendedHeader]) libhead.Getter[*header.ExtendedHeader] {
			return store
		}),
		fx.Provide(NewService),
		fx.Provide(func(serv *Service) Module {
			return serv
		}),
	)
}
