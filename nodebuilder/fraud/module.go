package fraud

import (
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

var log = logging.Logger("fraud-module")

func Module(tp node.Type) fx.Option {
	switch tp {
	case node.Light:
		return fx.Module(
			"fraud",
			fx.Provide(ServiceWithSyncer),
		)
	case node.Full, node.Bridge:
		return fx.Module(
			"fraud",
			fx.Provide(NewService),
		)
	default:
		panic("invalid node type")
	}
}
