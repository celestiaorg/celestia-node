package fraud

import (
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/libs/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

var log = logging.Logger("module/fraud")

func ConstructModule(tp node.Type) fx.Option {
	baseComponent := fx.Provide(func(serv fraud.Service) fraud.Getter {
		return serv
	})
	switch tp {
	case node.Light:
		return fx.Module(
			"fraud",
			baseComponent,
			fx.Provide(newFraudService(true)),
		)
	case node.Full, node.Bridge:
		return fx.Module(
			"fraud",
			baseComponent,
			fx.Provide(newFraudService(false)),
		)
	default:
		panic("invalid node type")
	}
}
