package header

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/fx"

	libhead "github.com/celestiaorg/go-header"
	headp2p "github.com/celestiaorg/go-header/p2p"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/pidstore"
	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

var log = logging.Logger("module/header")

// stubBroadcaster is a no-op broadcaster for when P2P is disabled.
type stubBroadcaster[H libhead.Header[H]] struct{}

func (s *stubBroadcaster[H]) Broadcast(_ context.Context, _ H, _ ...pubsub.PubOpt) error {
	return nil
}

// stubSubscriber is a no-op subscriber for when P2P is disabled.
type stubSubscriber[H libhead.Header[H]] struct{}

func (s *stubSubscriber[H]) Subscribe() (libhead.Subscription[H], error) {
	return &stubSubscription[H]{}, nil
}

func (s *stubSubscriber[H]) SetVerifier(func(context.Context, H) error) error {
	return nil
}

// stubSubscription is a no-op subscription for when P2P is disabled.
type stubSubscription[H libhead.Header[H]] struct{}

func (s *stubSubscription[H]) NextHeader(ctx context.Context) (H, error) {
	<-ctx.Done()
	var zero H
	return zero, ctx.Err()
}

func (s *stubSubscription[H]) Cancel() {
}

func ConstructModule[H libhead.Header[H]](tp node.Type, cfg *Config, p2pCfg *modp2p.Config) fx.Option {
	cfgErr := cfg.Validate(tp)

	p2pDisabled := p2pCfg != nil && p2pCfg.Disabled

	var headerServiceOption fx.Option
	if p2pDisabled {
		headerServiceOption = fx.Provide(fx.Annotate(
			newHeaderService,
			fx.ParamTags(``, ``, `optional:"true"`, ``, ``),
		))
	} else {
		headerServiceOption = fx.Provide(newHeaderService)
	}

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		headerServiceOption,
		fx.Provide(newStore[H]),
		fx.Provide(newSyncer[H]),
		fx.Provide(fx.Annotate(
			newFraudedSyncer[H],
			fx.OnStart(func(
				ctx context.Context,
				breaker *modfraud.ServiceBreaker[*sync.Syncer[H], H],
			) error {
				// TODO(@Wondertan): This fix flakes in e2e tests
				//  This is coming from the store asynchronity.
				//  Previously, we would request genesis during initialization
				//  but now we request it during Syncer start and given to the Store.
				//  However, the Store doesn't makes it immediately available causing flakes
				//  The proper fix will be in a follow up release after pruning.
				defer time.Sleep(time.Millisecond * 100)
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

	if !p2pDisabled {
		baseComponents = fx.Options(
			baseComponents,
			fx.Provide(func(subscriber *headp2p.Subscriber[H]) libhead.Subscriber[H] {
				return subscriber
			}),
			fx.Provide(fx.Annotate(
				func(ps *pubsub.PubSub, network modp2p.Network) (*headp2p.Subscriber[H], error) {
					opts := []headp2p.SubscriberOption{headp2p.WithSubscriberNetworkID(network.String())}
					if MetricsEnabled {
						opts = append(opts, headp2p.WithSubscriberMetrics())
					}
					return headp2p.NewSubscriber[H](ps, header.MsgID, opts...)
				},
				fx.OnStart(func(ctx context.Context, sub *headp2p.Subscriber[H]) error {
					return sub.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, sub *headp2p.Subscriber[H]) error {
					return sub.Stop(ctx)
				}),
			)),
			fx.Provide(fx.Annotate(
				func(
					cfg Config,
					host host.Host,
					store libhead.Store[H],
					network modp2p.Network,
				) (*headp2p.ExchangeServer[H], error) {
					opts := []headp2p.Option[headp2p.ServerParameters]{
						headp2p.WithParams(cfg.Server),
						headp2p.WithNetworkID[headp2p.ServerParameters](network.String()),
					}
					if MetricsEnabled {
						opts = append(opts, headp2p.WithMetrics[headp2p.ServerParameters]())
					}

					return headp2p.NewExchangeServer[H](host, store, opts...)
				},
				fx.OnStart(func(ctx context.Context, server *headp2p.ExchangeServer[H]) error {
					return server.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, server *headp2p.ExchangeServer[H]) error {
					return server.Stop(ctx)
				}),
			)),
		)
	}

	switch tp {
	case node.Light, node.Full:
		if p2pDisabled {
			return fx.Error(fmt.Errorf("p2p.disabled is only supported for Bridge nodes"))
		}
		return fx.Module(
			"header",
			baseComponents,
			fx.Provide(newP2PExchange[H]),
			fx.Provide(func(ctx context.Context, ds datastore.Batching) (headp2p.PeerIDStore, error) {
				return pidstore.NewPeerIDStore(ctx, ds)
			}),
		)
	case node.Bridge:
		if p2pDisabled {
			stubBroadcast := &stubBroadcaster[H]{}
			stubSub := &stubSubscriber[H]{}
			return fx.Module(
				"header",
				baseComponents,
				fx.Provide(func() libhead.Broadcaster[H] {
					return stubBroadcast
				}),
				fx.Provide(func() libhead.Subscriber[H] {
					return stubSub
				}),
				fx.Supply(header.MakeExtendedHeader),
			)
		}
		return fx.Module(
			"header",
			baseComponents,
			fx.Provide(func(subscriber *headp2p.Subscriber[H]) libhead.Broadcaster[H] {
				return subscriber
			}),
			fx.Supply(header.MakeExtendedHeader),
		)
	default:
		panic("invalid node type")
	}
}
