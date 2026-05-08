package fibre

import (
	"context"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"go.uber.org/fx"
	"google.golang.org/grpc"

	appfibre "github.com/celestiaorg/celestia-app/v9/fibre"
	appstate "github.com/celestiaorg/celestia-app/v9/fibre/state"
	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/fibre"
	"github.com/celestiaorg/celestia-node/fibre/stateclient"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/celestiaorg/celestia-node/state/txclient"
)

func ConstructModule(coreCfg *core.Config) fx.Option {
	enabled := coreCfg.IsEndpointConfigured()
	return fx.Module("fibre",
		fxutil.ProvideIf(enabled,
			fx.Annotate(
				newAppFibreClient,
				fx.OnStart(func(ctx context.Context, c *appfibre.Client) error {
					return c.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, c *appfibre.Client) error {
					return c.Stop(ctx)
				}),
			),
		),
		fxutil.ProvideIf(enabled,
			newFibreService,
			NewModule,
		),
		fxutil.ProvideIf(!enabled, func() (*fibre.Service, Module) {
			return nil, &stubbedFibreModule{}
		}),
	)
}

func newAppFibreClient(
	keyring keyring.Keyring,
	keyName state.AccountName,
	conn *grpc.ClientConn,
	tp node.Type,
	store libhead.Store[*header.ExtendedHeader],
	network p2p.Network,
) (*appfibre.Client, error) {
	cfg := appfibre.DefaultClientConfig()
	cfg.DefaultKeyName = string(keyName)
	cfg.StateAddress = conn.Target()

	if tp == node.Light {
		// Light nodes replace the default trusted gRPC state client with one
		// that resolves validator sets and hosts via the locally verified
		// header chain (see fibre/stateclient).
		sc := stateclient.NewClient(store, conn, network)
		cfg.StateClientFn = func() (appstate.Client, error) { return sc, nil }
	}
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
