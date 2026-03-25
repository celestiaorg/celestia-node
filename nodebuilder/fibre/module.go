package fibre

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/fibre"
	"github.com/celestiaorg/celestia-node/state/txclient"
)

func ConstructModule() fx.Option {
	return fx.Module("fibre",
		fx.Provide(func(params struct {
			fx.In
			TxClient *txclient.TxClient `optional:"true"`
		},
		) (*fibre.Client, Module) {
			if params.TxClient == nil {
				return nil, &stubbedFibreModule{}
			}
			client := fibre.NewClient(params.TxClient)
			serv := NewService(client)
			return client, serv
		}),
	)
}
