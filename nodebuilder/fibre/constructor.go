package fibre

import (
	"context"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"go.uber.org/fx"
	"google.golang.org/grpc"

	appfibre "github.com/celestiaorg/celestia-app/v9/fibre"

	"github.com/celestiaorg/celestia-node/fibre"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/celestiaorg/celestia-node/state/txclient"
)

func ConstructModule(coreCfg *core.Config) fx.Option {
	return fx.Module("fibre",
		fxutil.ProvideIf(coreCfg.IsEndpointConfigured(),
			fx.Annotate(
				newAppFibreClient,
				fx.OnStart(func(ctx context.Context, c *appfibre.Client) error {
					return c.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, c *appfibre.Client) error {
					return c.Stop(ctx)
				}),
			),
			newFibreService,
			NewModule,
		),
		fxutil.ProvideIf(!coreCfg.IsEndpointConfigured(), func() (*fibre.Service, Module) {
			return nil, &stubbedFibreModule{}
		}),
	)
}

func newAppFibreClient(
	keyring keyring.Keyring,
	keyName state.AccountName,
	conn *grpc.ClientConn,
) (*appfibre.Client, error) {
	cfg := appfibre.DefaultClientConfig()
	cfg.DefaultKeyName = string(keyName)
	cfg.StateAddress = conn.Target()
	return appfibre.NewClient(keyring, cfg)
}

func newFibreService(
	appClient *appfibre.Client,
	tc *txclient.TxClient,
	client *grpc.ClientConn,
	additionalConns core.AdditionalCoreConns,
) *fibre.Service {
	acc := fibre.NewAccountClient(tc, client, fibre.WithAdditionalConnections(additionalConns...))
	return fibre.NewService(appClient, tc, acc)
}
