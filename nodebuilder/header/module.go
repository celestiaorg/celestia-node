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
	fraudServ "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/params"
)

var log = logging.Logger("header-module")

func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Supply(params.BlockTime),
		fx.Provide(NewHeaderService),
		fx.Provide(fx.Annotate(
			store.NewStore,
			fx.OnStart(func(ctx context.Context, store header.Store) error {
				return store.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, store header.Store) error {
				return store.Stop(ctx)
			}),
		)),
		fx.Provide(newInitStore),
		fx.Provide(func(subscriber *p2p.Subscriber) header.Subscriber {
			return subscriber
		}),
		fx.Provide(fx.Annotate(
			newSyncer,
			fx.OnStart(func(
				startCtx context.Context,
				ctx context.Context,
				fservice fraudServ.Module,
				syncer *sync.Syncer,
			) error {
				syncerStartFunc := func(ctx context.Context) error {
					err := syncer.Start(ctx)
					switch err {
					default:
						return err
					case header.ErrNoHead:
						log.Warnw("Syncer running on uninitialized Store - headers won't be synced")
					case nil:
					}
					return nil
				}
				return fraudServ.Lifecycle(startCtx, ctx, fraud.BadEncoding, fservice,
					syncerStartFunc, syncer.Stop)
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
	case node.Light, node.Full:
		return fx.Module(
			"header",
			baseComponents,
			fx.Provide(newP2PExchange(*cfg)),
		)
	case node.Bridge:
		return fx.Module(
			"header",
			baseComponents,
			fx.Provide(func(subscriber *p2p.Subscriber) header.Broadcaster {
				return subscriber
			}),
			fx.Supply(header.MakeExtendedHeader),
		)
	default:
		panic("invalid node type")
	}
}
