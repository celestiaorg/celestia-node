package header

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/p2p"
	"github.com/celestiaorg/celestia-node/header/store"
	"github.com/celestiaorg/celestia-node/header/sync"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/params"
	headerservice "github.com/celestiaorg/celestia-node/service/header"
)

var log = logging.Logger("header-module")

func Module(tp node.Type, cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.ValidateBasic()

	baseOptions := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Supply(params.BlockTime),
		fx.Provide(headerservice.NewHeaderService),
		fx.Provide(fx.Annotate(
			store.NewStore,
			fx.OnStart(func(ctx context.Context, store header.Store) error {
				return store.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, store header.Store) error {
				return store.Stop(ctx)
			}),
		)),
		fx.Invoke(InitStore),
		fx.Provide(func(subscriber *p2p.Subscriber) header.Subscriber {
			return subscriber
		}),
		fx.Provide(func(subscriber *p2p.Subscriber) header.Broadcaster {
			return subscriber
		}),
		fx.Provide(fx.Annotate(
			sync.NewSyncer,
			fx.OnStart(func(ctx context.Context, lc fx.Lifecycle, fservice fraud.Service, syncer *sync.Syncer) error {
				lifecycleCtx := fxutil.WithLifecycle(ctx, lc)
				return FraudLifecycle(ctx, lifecycleCtx, fraud.BadEncoding, fservice, syncer.Start, syncer.Stop)
			}),
			fx.OnStop(func(ctx context.Context, syncer *sync.Syncer) error {
				return syncer.Stop(ctx)
			}),
		)),
		fx.Provide(fx.Annotate(
			p2p.NewSubscriber,
			fx.OnStart(func(ctx context.Context, sub *p2p.Subscriber) error {
				return sub.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, sub *p2p.Subscriber) error {
				return sub.Stop(ctx)
			}),
		)),

		fx.Provide(fx.Annotate(
			p2p.NewExchangeServer,
			fx.OnStart(func(ctx context.Context, server *p2p.ExchangeServer) error {
				return server.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, server *p2p.ExchangeServer) error {
				return server.Stop(ctx)
			}),
		)),
	)

	switch tp {
	case node.Light:
		return fx.Module(
			"header",
			baseOptions,
			fx.Provide(P2PExchange(*cfg)),
			fxutil.ProvideAs(FraudServiceWithSyncer, new(fraud.Service), new(fraud.Subscriber)),
		)
	case node.Full:
		return fx.Module(
			"header",
			baseOptions,
			fx.Provide(P2PExchange(*cfg)),
			fxutil.ProvideAs(FraudService, new(fraud.Service), new(fraud.Subscriber)),
		)
	case node.Bridge:
		return fx.Module(
			"header",
			baseOptions,
			fx.Supply(header.MakeExtendedHeader),
			fxutil.ProvideAs(FraudService, new(fraud.Service), new(fraud.Subscriber)),
		)
	default:
		panic("invalid node type")
	}
}
