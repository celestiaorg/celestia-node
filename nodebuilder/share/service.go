package share

import (
	"context"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share/service"

	"github.com/ipfs/go-blockservice"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/share"
)

type module struct {
	share.Availability
	service.ShareService
}

func NewShareService(lc fx.Lifecycle, bServ blockservice.BlockService) service.ShareService {
	serv := service.NewShareService(bServ)
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return serv.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return serv.Stop(ctx)
		},
	})
	return serv
}

func NewModule(lc fx.Lifecycle, tp node.Type, shrSrv service.ShareService, availability share.Availability) Module {
	return newModule(shrSrv, availability)
}
