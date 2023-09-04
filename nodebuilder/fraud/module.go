package fraud

import (
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/go-fraud"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

var log = logging.Logger("module/fraud")

func ConstructModule(tp node.Type) fx.Option {
	baseComponent := fx.Options(
		fx.Provide(fraudUnmarshaler),
		fx.Provide(func(serv fraud.Service[*header.ExtendedHeader]) fraud.Getter[*header.ExtendedHeader] {
			return serv
		}),
	)
	switch tp {
	case node.Light:
		return fx.Module(
			"fraud",
			baseComponent,
			fx.Provide(newFraudServiceWithSync),
		)
	case node.Full, node.Bridge:
		return fx.Module(
			"fraud",
			baseComponent,
			fx.Provide(newFraudServiceWithoutSync),
		)
	default:
		panic("invalid node type")
	}
}
