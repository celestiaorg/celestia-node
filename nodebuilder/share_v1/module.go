package share_v1

import (
	"go.uber.org/fx"

	headerServ "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func ConstructModule(tp node.Type, options ...fx.Option) fx.Option {
	baseComponents := fx.Options(
		fx.Options(options...),
		fx.Provide(newShareV1Module),
	)

	return fx.Module(
		"share_v1",
		baseComponents,
	)
}

func newShareV1Module(
	getter shwap.Getter,
	avail share.Availability,
	hs headerServ.Module,
) Module {
	return &module{
		getter: getter,
		avail:  avail,
		hs:     hs,
	}
}
