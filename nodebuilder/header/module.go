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
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/pidstore"
	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

var log = logging.Logger("module/header")

func ConstructModule[H libhead.Header[H]](tp node.Type, cfg *Config, p2pCfg *modp2p.Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate(tp)

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Provide(newHeaderService),
		fx.Provide(newInitStore[H]),
		fx.Provide(newSyncer[H]),
		fx.Provide(fx.Annotate(
			newFraudedSyncer[H],
			fx.OnStart(func(
				ctx context.Context,
				breaker *modfraud.ServiceBreaker[*sync.Syncer[H], H],
			) error {
				return breaker.Start(ctx)
			}),
			fx.OnStop(func(
				ctx context.Context,
				breaker *modfraud.ServiceBreaker[*sync.Syncer[H], H],
			) error {
				return breaker.Stop(ctx)
			}),
		)),
	)

	// Conditionally add p2p components only if p2p is enabled
	var p2pComponents fx.Option
	if !p2pCfg.DisableP2P {
		p2pComponents = fx.Options(
			fx.Provide(fx.Annotate(
				func(ps *pubsub.PubSub, network modp2p.Network) (*p2p.Subscriber[H], error) {
					opts := []p2p.SubscriberOption{p2p.WithSubscriberNetworkID(network.String())}
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
			)),
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
		)
	} else {
		// When p2p is disabled, provide stub components
		p2pComponents = fx.Options(
			fx.Provide(func() *p2p.Subscriber[H] { return nil }),
			fx.Provide(func() *p2p.ExchangeServer[H] { return nil }),
		)
	}

	allComponents := fx.Options(
		baseComponents,
		p2pComponents,
		fx.Provide(func(subscriber *p2p.Subscriber[H]) libhead.Subscriber[H] {
			if subscriber == nil {
				return nil
			}
			return subscriber
		}),
	)

	switch tp {
	case node.Light, node.Full:
		return fx.Module(
			"header",
			allComponents,
			fx.Provide(newP2PExchange[H]),
			fx.Provide(func(ctx context.Context, ds datastore.Batching) (p2p.PeerIDStore, error) {
				return pidstore.NewPeerIDStore(ctx, ds)
			}),
		)
	case node.Bridge:
		return fx.Module(
			"header",
			allComponents,
			fx.Provide(func(subscriber *p2p.Subscriber[H]) libhead.Broadcaster[H] {
				if subscriber == nil {
					return nil
				}
				return subscriber
			}),
			fx.Supply(header.MakeExtendedHeader),
		)
	default:
		panic("invalid node type")
	}
}
