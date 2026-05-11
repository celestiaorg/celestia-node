package header

import (
	"context"
	"time"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/fx"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/p2p"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/pidstore"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

var log = logging.Logger("module/header")

// newSubscriber constructs the fx.Option for the p2p.Subscriber component.
// Bridge nodes use FanoutOnly mode to prevent receiving gossip headers before
// the local core.Listener has finished storing the corresponding EDS.
func newSubscriber[H libhead.Header[H]](tp node.Type) fx.Option {
	return fx.Provide(fx.Annotate(
		func(ps *pubsub.PubSub, network modp2p.Network) (*p2p.Subscriber[H], error) {
			opts := []p2p.SubscriberOption{p2p.WithSubscriberNetworkID(network.String())}
			// Bridge nodes must not receive headers via p2p gossip: a remote peer
			// can gossip the same header before the local core.Listener finishes
			// storing EDS, causing the DASer to access EDS prematurely.
			if tp == node.Bridge {
				opts = append(opts, p2p.WithTopicOpts(pubsub.FanoutOnly()))
			}
			if MetricsEnabled {
				opts = append(opts, p2p.WithSubscriberMetrics())
			}
			return p2p.NewSubscriber[H](ps, header.MsgID, opts...)
		},
		fx.OnStart(func(ctx context.Context, sub *p2p.Subscriber[H]) error {
			return sub.Start(ctx)
		}),
		fx.OnStop(func(ctx context.Context, sub *p2p.Subscriber[H]) error {
			return sub.Stop(ctx)
		}),
	))
}

func ConstructModule[H libhead.Header[H]](tp node.Type, cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate(tp)

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Provide(newHeaderService),
		fx.Provide(newStore[H]),
		fx.Provide(func(subscriber *p2p.Subscriber[H]) libhead.Subscriber[H] {
			return subscriber
		}),
		fx.Provide(fx.Annotate(
			newSyncer[H],
			fx.OnStart(func(
				ctx context.Context,
				syncer *sync.Syncer[H],
			) error {
				// TODO(@Wondertan): This fix flakes in e2e tests
				//  This is coming from the store asynchronity.
				//  Previously, we would request genesis during initialization
				//  but now we request it during Syncer start and given to the Store.
				//  However, the Store doesn't makes it immediately available causing flakes
				//  The proper fix will be in a follow up release after pruning.
				defer time.Sleep(time.Millisecond * 100)
				return syncer.Start(ctx)
			}),
			fx.OnStop(func(
				ctx context.Context,
				syncer *sync.Syncer[H],
			) error {
				return syncer.Stop(ctx)
			}),
		)),
		newSubscriber[H](tp),
		fx.Provide(fx.Annotate(
			func(
				cfg Config,
				host host.Host,
				store libhead.Store[H],
				network modp2p.Network,
			) (*p2p.ExchangeServer[H], error) {
				opts := []p2p.Option[p2p.ServerParameters]{
					p2p.WithParams(cfg.Server),
					p2p.WithNetworkID[p2p.ServerParameters](network.String()),
				}
				if MetricsEnabled {
					opts = append(opts, p2p.WithMetrics[p2p.ServerParameters]())
				}

				return p2p.NewExchangeServer[H](host, store, opts...)
			},
			fx.OnStart(func(ctx context.Context, server *p2p.ExchangeServer[H]) error {
				return server.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, server *p2p.ExchangeServer[H]) error {
				return server.Stop(ctx)
			}),
		)),
		fx.Provide(newP2PExchange[H]),
		fx.Provide(func(ctx context.Context, ds datastore.Batching) (p2p.PeerIDStore, error) {
			return pidstore.NewPeerIDStore(ctx, ds)
		}),
	)

	switch tp {
	case node.Light:
		return fx.Module(
			"header",
			baseComponents,
			fx.Provide(func(ex *p2p.Exchange[H]) libhead.Exchange[H] {
				return ex
			}),
		)
	case node.Bridge:
		return fx.Module(
			"header",
			baseComponents,
			fx.Provide(func(subscriber *p2p.Subscriber[H]) libhead.Broadcaster[H] {
				return subscriber
			}),
			fx.Supply(header.MakeExtendedHeader),
		)
	default:
		panic("invalid node type")
	}
}
