package node

import (
	"context"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/raulk/go-watchdog"
	"go.uber.org/fx"

	nodecore "github.com/celestiaorg/celestia-node/node/core"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/node/p2p"
	"github.com/celestiaorg/celestia-node/node/services"
	"github.com/celestiaorg/celestia-node/service/header"
)

// lightComponents keeps all the components as DI options required to built a Light Node.
func lightComponents(cfg *Config, store Store) fxutil.Option {
	return fxutil.Options(
		fxutil.Supply(Light),
		baseComponents(cfg, store),
		fxutil.Provide(services.DASer),
		fxutil.Provide(services.HeaderExchangeP2P(cfg.Services)),
	)
}

// bridgeComponents keeps all the components as DI options required to build a Bridge Node.
func bridgeComponents(cfg *Config, store Store) fxutil.Option {
	return fxutil.Options(
		fxutil.Supply(Bridge),
		baseComponents(cfg, store),
		nodecore.Components(cfg.Core, store.Core),
	)
}

// baseComponents keeps all the common components shared between different Node types.
func baseComponents(cfg *Config, store Store) fxutil.Option {
	return fxutil.Options(
		fxutil.Provide(context.Background),
		fxutil.Supply(cfg),
		fxutil.Supply(store.Config),
		fxutil.Provide(store.Datastore),
		fxutil.Provide(store.Keystore),
		fxutil.Provide(services.ShareService),
		// TODO @renaynay: once full node type is defined, add FullAvailability
		//  to full node and LightAvailability to light node. Bridge node does
		//  not need Availability.
		fxutil.Provide(services.LightAvailability),
		fxutil.Provide(services.HeaderService),
		fxutil.Provide(services.HeaderStore),
		fxutil.Invoke(services.HeaderStoreInit(&cfg.Services)),
		fxutil.Provide(services.HeaderSyncer),
		fxutil.ProvideAs(services.P2PSubscriber, new(header.Broadcaster), new(header.Subscriber)),
		fxutil.Provide(services.HeaderP2PExchangeServer),
		fxutil.Invoke(invkWatchdog(store.Path())),
		p2p.Components(cfg.P2P),
	)
}

func invkWatchdog(pprofdir string) func(lc fx.Lifecycle) error {
	return func(lc fx.Lifecycle) error {
		// to get watchdog information logged out
		watchdog.Logger = logWatchdog
		// these set up heap pprof auto capturing on disk when threshold hit 90% usage
		watchdog.HeapProfileDir = pprofdir
		watchdog.HeapProfileMaxCaptures = 10
		watchdog.HeapProfileThreshold = 0.9

		policy := watchdog.NewWatermarkPolicy(0.50, 0.60, 0.70, 0.85, 0.90, 0.925, 0.95)
		err, stop := watchdog.SystemDriven(0, time.Second*5, policy)
		if err != nil {
			return err
		}
		lc.Append(fx.Hook{
			OnStop: func(context.Context) error {
				stop()
				return nil
			},
		})
		return nil
	}
}

var logWatchdog = logging.Logger("watchdog")
