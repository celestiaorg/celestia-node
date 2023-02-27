package header

import (
	"context"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/libs/header/p2p"
	"github.com/celestiaorg/celestia-node/libs/header/store"
	"github.com/celestiaorg/celestia-node/libs/header/sync"
	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
)

var log = logging.Logger("module/header")

func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate(tp)

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
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
			func(cfg Config, network modp2p.Network) []p2p.Option[p2p.ServerParameters] {
				return []p2p.Option[p2p.ServerParameters]{
					p2p.WithWriteDeadline(cfg.Server.WriteDeadline),
					p2p.WithReadDeadline(cfg.Server.ReadDeadline),
					p2p.WithMaxRequestSize[p2p.ServerParameters](cfg.Server.MaxRequestSize),
					p2p.WithRequestTimeout[p2p.ServerParameters](cfg.Server.RequestTimeout),
					p2p.WithNetworkID[p2p.ServerParameters](network.String()),
				}
			}),
		fx.Provide(newHeaderService),
		fx.Provide(fx.Annotate(
			func(ds datastore.Batching, opts []store.Option) (libhead.Store[*header.ExtendedHeader], error) {
				return store.NewStore[*header.ExtendedHeader](ds, opts...)
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
		fx.Provide(func(cfg Config) []sync.Options {
			return []sync.Options{
				sync.WithBlockTime(modp2p.BlockTime),
				sync.WithTrustingPeriod(cfg.Syncer.TrustingPeriod),
				sync.WithMaxRequestSize(cfg.Syncer.MaxRequestSize),
			}
		}),
		fx.Provide(fx.Annotate(
			newSyncer,
			fx.OnStart(func(
				startCtx, ctx context.Context,
				fservice fraud.Service,
				syncer *sync.Syncer[*header.ExtendedHeader],
			) error {
				return modfraud.Lifecycle(startCtx, ctx, byzantine.BadEncoding, fservice,
					syncer.Start, syncer.Stop)
			}),
			fx.OnStop(func(ctx context.Context, syncer *sync.Syncer[*header.ExtendedHeader]) error {
				return syncer.Stop(ctx)
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
				func(cfg Config, network modp2p.Network) []p2p.Option[p2p.ClientParameters] {
					return []p2p.Option[p2p.ClientParameters]{
						p2p.WithMinResponses(cfg.Client.MinResponses),
						p2p.WithMaxRequestSize[p2p.ClientParameters](cfg.Client.MaxRequestSize),
						p2p.WithMaxHeadersPerRequest(cfg.Client.MaxHeadersPerRequest),
						p2p.WithMaxAwaitingTime(cfg.Client.MaxAwaitingTime),
						p2p.WithDefaultScore(cfg.Client.DefaultScore),
						p2p.WithRequestTimeout[p2p.ClientParameters](cfg.Client.RequestTimeout),
						p2p.WithMaxTrackerSize(cfg.Client.MaxPeerTrackerSize),
						p2p.WithNetworkID[p2p.ClientParameters](network.String()),
						p2p.WithChainID(network.String()),
					}
				},
			),
			fx.Provide(newP2PExchange(*cfg)),
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
