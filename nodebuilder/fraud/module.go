package fraud

import (
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

var log = logging.Logger("module/fraud")

func ConstructModule(tp node.Type) fx.Option {
	baseComponent := fx.Provide(func(module Module) fraud.Getter {
		return module
	})
	switch tp {
	case node.Light:
		return fx.Module(
			"fraud",
			baseComponent,
			fx.Provide(ModuleWithSyncer),
		)
	case node.Full, node.Bridge:
		return fx.Module(
			"fraud",
			baseComponent,
			fx.Provide(NewModule),
		)
	default:
		panic("invalid node type")
	}
}
