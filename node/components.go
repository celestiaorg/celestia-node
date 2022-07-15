package node

import (
	"context"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/raulk/go-watchdog"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	nodecore "github.com/celestiaorg/celestia-node/node/core"
	"github.com/celestiaorg/celestia-node/node/p2p"
	"github.com/celestiaorg/celestia-node/node/rpc"
	"github.com/celestiaorg/celestia-node/node/services"
	statecomponents "github.com/celestiaorg/celestia-node/node/state"
	"github.com/celestiaorg/celestia-node/params"
	headerServ "github.com/celestiaorg/celestia-node/service/header"
	rpcServ "github.com/celestiaorg/celestia-node/service/rpc"
	"github.com/celestiaorg/celestia-node/service/share"
	"github.com/celestiaorg/celestia-node/service/state"
)

// lightComponents keeps all the components as DI options required to build a Light Node.
func lightComponents(cfg *Config, store Store) fx.Option {
	return fx.Options(
		fx.Supply(Light),
		baseComponents(cfg, store),
		fx.Provide(services.DASer),
		fx.Provide(services.HeaderExchangeP2P(cfg.Services)),
		fx.Provide(services.LightAvailability),
		fx.Invoke(rpc.Handler),
	)
}

// bridgeComponents keeps all the components as DI options required to build a Bridge Node.
func bridgeComponents(cfg *Config, store Store) fx.Option {
	return fx.Options(
		fx.Supply(Bridge),
		baseComponents(cfg, store),
		nodecore.Components(cfg.Core),
		fx.Supply(header.MakeExtendedHeader),
		fx.Provide(services.LightAvailability), // TODO(@Wondertan): Remove strict requirements to have Availability
		fx.Invoke(func(
			state *state.Service,
			share *share.Service,
			header *headerServ.Service,
			rpcSrv *rpcServ.Server,
		) {
			rpc.Handler(state, share, header, rpcSrv, nil)
		}),
	)
}

// fullComponents keeps all the components as DI options required to build a Full Node.
func fullComponents(cfg *Config, store Store) fx.Option {
	return fx.Options(
		fx.Supply(Full),
		baseComponents(cfg, store),
		fx.Provide(services.DASer),
		fx.Provide(services.HeaderExchangeP2P(cfg.Services)),
		fx.Provide(services.FullAvailability),
		fx.Invoke(rpc.Handler),
	)
}

// baseComponents keeps all the common components shared between different Node types.
func baseComponents(cfg *Config, store Store) fx.Option {
	return fx.Options(
		fx.Provide(params.DefaultNetwork),
		fx.Provide(params.BootstrappersFor),
		fx.Provide(context.Background),
		fx.Supply(cfg),
		fx.Supply(store.Config),
		fx.Provide(store.Datastore),
		fx.Provide(store.Keystore),
		// share components
		fx.Invoke(share.EnsureEmptySquareExists),
		fx.Provide(services.ShareService),
		// header components
		fx.Provide(services.HeaderService),
		fx.Provide(services.HeaderStore),
		fx.Invoke(services.HeaderStoreInit(&cfg.Services)),
		fxutil.ProvideAs(services.FraudService, new(fraud.Service), new(fraud.Subscriber)),
		fx.Provide(services.HeaderSyncer),
		fxutil.ProvideAs(services.P2PSubscriber, new(header.Broadcaster), new(header.Subscriber)),
		fx.Provide(services.HeaderP2PExchangeServer),
		// p2p components
		fx.Invoke(invokeWatchdog(store.Path())),
		p2p.Components(cfg.P2P),
		// state components
		statecomponents.Components(cfg.Core, cfg.Key),
		// RPC components
		fx.Provide(rpc.Server(cfg.RPC)),
	)
}

// invokeWatchdog starts the memory watchdog that helps to prevent some of OOMs by forcing GCing
// It also collects heap profiles in the given directory when heap grows to more than 90% of memory usage
func invokeWatchdog(pprofdir string) func(lc fx.Lifecycle) error {
	return func(lc fx.Lifecycle) (errOut error) {
		onceWatchdog.Do(func() {
			// to get watchdog information logged out
			watchdog.Logger = logWatchdog
			// these set up heap pprof auto capturing on disk when threshold hit 90% usage
			watchdog.HeapProfileDir = pprofdir
			watchdog.HeapProfileMaxCaptures = 10
			watchdog.HeapProfileThreshold = 0.9

			policy := watchdog.NewWatermarkPolicy(0.50, 0.60, 0.70, 0.85, 0.90, 0.925, 0.95)
			err, stop := watchdog.SystemDriven(0, time.Second*5, policy)
			if err != nil {
				errOut = err
				return
			}

			lc.Append(fx.Hook{
				OnStop: func(context.Context) error {
					stop()
					return nil
				},
			})
		})
		return
	}
}

// TODO(@Wondetan): We must start watchdog only once. This is needed for tests where we run multiple instance
//  of the Node. Ideally, the Node should have some testing options instead, so we can check for it and run without
//  such utilities but it does not hurt to run one instance of watchdog per test.
var onceWatchdog = sync.Once{}

var logWatchdog = logging.Logger("watchdog")
