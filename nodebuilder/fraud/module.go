package fraud

import (
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

var log = logging.Logger("fraud-module")

func Module(tp node.Type) fx.Option {
	switch tp {
	case node.Light:
		return fx.Module(
			"fraud",
			fxutil.ProvideAs(ServiceWithSyncer, new(fraud.Service), new(fraud.Subscriber)),
		)
	case node.Full, node.Bridge:
		return fx.Module(
			"fraud",
			fxutil.ProvideAs(Service, new(fraud.Service), new(fraud.Subscriber)),
		)
	default:
		panic("invalid node type")
	}
}
