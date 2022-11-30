package header

import (
	"context"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/p2p"
	"github.com/celestiaorg/celestia-node/header/store"
	"github.com/celestiaorg/celestia-node/header/sync"
	fraudServ "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
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
					p2p.WithWriteDeadline(cfg.ServerParameters.WriteDeadline),
					p2p.WithReadDeadline(cfg.ServerParameters.ReadDeadline),
					p2p.WithMaxRequestSize[p2p.ServerParameters](cfg.ServerParameters.MaxRequestSize),
				}
			}),
		fx.Provide(NewHeaderService),
		fx.Provide(fx.Annotate(
			func(ds datastore.Batching, opts []store.Option) (header.Store, error) {
				return store.NewStore(ds, opts...)
			},
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
			fx.OnStart(func(startCtx, ctx context.Context, fservice fraud.Service, syncer *sync.Syncer) error {
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
			newP2PServer,
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
			fx.Provide(
				func(cfg Config) []p2p.Option[p2p.ClientParameters] {
					return []p2p.Option[p2p.ClientParameters]{
						p2p.WithMinResponses(cfg.ClientParameters.MinResponses),
						p2p.WithMaxRequestSize[p2p.ClientParameters](cfg.ClientParameters.MaxRequestSize),
						p2p.WithMaxHeadersPerRequest(cfg.ClientParameters.MaxHeadersPerRequest),
					}
				},
			),
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
