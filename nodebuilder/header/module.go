package header

import (
	"context"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/pkg/header/p2p"
	"github.com/celestiaorg/celestia-node/pkg/header/store"
	"github.com/celestiaorg/celestia-node/pkg/header/sync"
)

var log = logging.Logger("module/header")

func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate(tp)

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Supply(modp2p.BlockTime),
		fx.Provide(
			func(cfg Config) []store.Option {
				return []store.Option{
					store.WithStoreCacheSize(cfg.Store.StoreCacheSize),
					store.WithIndexCacheSize(cfg.Store.IndexCacheSize),
					store.WithWriteBatchSize(cfg.Store.WriteBatchSize),
				}
			},
		),
		fx.Provide(
			func(cfg Config) []p2p.Option[p2p.ServerParameters] {
				return []p2p.Option[p2p.ServerParameters]{
					p2p.WithWriteDeadline(cfg.Server.WriteDeadline),
					p2p.WithReadDeadline(cfg.Server.ReadDeadline),
					p2p.WithMaxRequestSize[p2p.ServerParameters](cfg.Server.MaxRequestSize),
					p2p.WithRequestTimeout[p2p.ServerParameters](cfg.Server.RequestTimeout),
				}
			}),
		fx.Provide(NewHeaderService),
		fx.Provide(fx.Annotate(
			func(ds datastore.Batching, opts []store.Option) (header.Store, error) {
				return store.NewStore[*header.ExtendedHeader](ds, opts...)
			},
			fx.OnStart(func(ctx context.Context, store header.Store) error {
				return store.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, store header.Store) error {
				return store.Stop(ctx)
			}),
		)),
		fx.Provide(newInitStore),
		fx.Provide(func(subscriber *p2p.Subscriber[*header.ExtendedHeader]) header.Subscriber {
			return subscriber
		}),
		fx.Provide(func(cfg Config) []sync.Options {
			return []sync.Options{
				sync.WithBlockTime(modp2p.BlockTime),
				sync.WithTrustingPeriod(cfg.Syncer.TrustingPeriod),
				sync.WithMaxRequestSize(cfg.Syncer.MaxRequestSize),
			}
		}),
		fx.Provide(fx.Annotate(
			newSyncer,
			fx.OnStart(func(startCtx, ctx context.Context, fservice fraud.Service, syncer *sync.Syncer[*header.ExtendedHeader]) error {
				return modfraud.Lifecycle(startCtx, ctx, fraud.BadEncoding, fservice,
					syncer.Start, syncer.Stop)
			}),
			fx.OnStop(func(ctx context.Context, syncer *sync.Syncer[*header.ExtendedHeader]) error {
				return syncer.Stop(ctx)
			}),
		)),
		fx.Provide(fx.Annotate(
			p2p.NewSubscriber[*header.ExtendedHeader],
			fx.OnStart(func(ctx context.Context, sub *p2p.Subscriber[*header.ExtendedHeader]) error {
				return sub.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, sub *p2p.Subscriber[*header.ExtendedHeader]) error {
				return sub.Stop(ctx)
			}),
		)),
		fx.Provide(fx.Annotate(
			newP2PServer,
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
			fx.Provide(
				func(cfg Config) []p2p.Option[p2p.ClientParameters] {
					return []p2p.Option[p2p.ClientParameters]{
						p2p.WithMinResponses(cfg.Client.MinResponses),
						p2p.WithMaxRequestSize[p2p.ClientParameters](cfg.Client.MaxRequestSize),
						p2p.WithMaxHeadersPerRequest(cfg.Client.MaxHeadersPerRequest),
						p2p.WithMaxAwaitingTime(cfg.Client.MaxAwaitingTime),
						p2p.WithDefaultScore(cfg.Client.DefaultScore),
						p2p.WithRequestTimeout[p2p.ClientParameters](cfg.Client.RequestTimeout),
						p2p.WithMaxTrackerSize(cfg.Client.MaxPeerTrackerSize),
					}
				},
			),
			fx.Provide(newP2PExchange(*cfg)),
		)
	case node.Bridge:
		return fx.Module(
			"header",
			baseComponents,
			fx.Provide(func(subscriber *p2p.Subscriber[*header.ExtendedHeader]) header.Broadcaster {
				return subscriber
			}),
			fx.Supply(header.MakeExtendedHeader),
		)
	default:
		panic("invalid node type")
	}
}
