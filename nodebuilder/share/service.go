package share

import (
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
	return serv
}

func NewModule(lc fx.Lifecycle, tp node.Type, shrSrv service.ShareService, availability share.Availability) Module {
	return newModule(shrSrv, availability)
}
