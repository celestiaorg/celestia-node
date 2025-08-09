package fraud

import (
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/go-fraud"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

var log = logging.Logger("module/fraud")

func ConstructModule(tp node.Type, p2pCfg *p2p.Config) fx.Option {
	baseComponent := fx.Options(
		fx.Provide(Unmarshaler),
	)
	
	// Only provide fraud service if p2p is enabled
	if !p2pCfg.DisableP2P {
		baseComponent = fx.Options(
			baseComponent,
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
	
	// If p2p is disabled, provide a minimal fraud module with stub service
	return fx.Module(
		"fraud",
		baseComponent,
		fx.Provide(newStubFraudService),
		fx.Provide(func(serv fraud.Service[*header.ExtendedHeader]) fraud.Getter[*header.ExtendedHeader] {
			return serv
		}),
	)
}
