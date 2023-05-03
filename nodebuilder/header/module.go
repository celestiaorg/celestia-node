package header

import (
	"context"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/fx"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/p2p"
	"github.com/celestiaorg/go-header/store"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

var log = logging.Logger("module/header")

func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate(tp)

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Provide(newHeaderService),
		fx.Provide(fx.Annotate(
			func(ds datastore.Batching) (libhead.Store[*header.ExtendedHeader], error) {
				return store.NewStore[*header.ExtendedHeader](ds, store.WithParams(cfg.Store))
			},
			fx.OnStart(func(ctx context.Context, store libhead.Store[*header.ExtendedHeader]) error {
				return store.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, store libhead.Store[*header.ExtendedHeader]) error {
				return store.Stop(ctx)
			}),
		)),
		fx.Provide(newInitStore),
		fx.Provide(func(subscriber *p2p.Subscriber[*header.ExtendedHeader]) libhead.Subscriber[*header.ExtendedHeader] {
			return subscriber
		}),
		fx.Provide(fx.Annotate(
			newSyncer,
			fx.OnStart(func(
				ctx context.Context,
				breaker *modfraud.ServiceBreaker[*sync.Syncer[*header.ExtendedHeader]],
			) error {
				return breaker.Start(ctx)
			}),
			fx.OnStop(func(
				ctx context.Context,
				breaker *modfraud.ServiceBreaker[*sync.Syncer[*header.ExtendedHeader]],
			) error {
				return breaker.Stop(ctx)
			}),
		)),
		fx.Provide(fx.Annotate(
			func(ps *pubsub.PubSub, network modp2p.Network) *p2p.Subscriber[*header.ExtendedHeader] {
				return p2p.NewSubscriber[*header.ExtendedHeader](ps, header.MsgID, network.String())
			},
			fx.OnStart(func(ctx context.Context, sub *p2p.Subscriber[*header.ExtendedHeader]) error {
				return sub.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, sub *p2p.Subscriber[*header.ExtendedHeader]) error {
				return sub.Stop(ctx)
			}),
		)),
		fx.Provide(fx.Annotate(
			func(
				host host.Host,
				store libhead.Store[*header.ExtendedHeader],
				network modp2p.Network,
			) (*p2p.ExchangeServer[*header.ExtendedHeader], error) {
				return p2p.NewExchangeServer[*header.ExtendedHeader](host, store,
					p2p.WithParams(cfg.Server),
					p2p.WithNetworkID[p2p.ServerParameters](network.String()),
				)
			},
			fx.OnStart(func(ctx context.Context, server *p2p.ExchangeServer[*header.ExtendedHeader]) error {
				return server.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, server *p2p.ExchangeServer[*header.ExtendedHeader]) error {
				return server.Stop(ctx)
			}),
		)),
	)

	switch tp {
	case node.Light, node.Full:
		return fx.Module(
			"header",
			baseComponents,
			fx.Provide(newP2PExchange),
		)
	case node.Bridge:
		return fx.Module(
			"header",
			baseComponents,
			fx.Provide(func(subscriber *p2p.Subscriber[*header.ExtendedHeader]) libhead.Broadcaster[*header.ExtendedHeader] {
				return subscriber
			}),
			fx.Supply(header.MakeExtendedHeader),
		)
	default:
		panic("invalid node type")
	}
}
