package p2p

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

var log = logging.Logger("module/p2p")

// ConstructModule collects all the components and services related to p2p.
func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Provide(Key),
		fx.Provide(id),
		fx.Provide(peerStore),
		fx.Provide(connectionManager),
		fx.Provide(connectionGater),
		fx.Provide(host),
		fx.Provide(routedHost),
		fx.Provide(pubSub),
		fx.Provide(dataExchange),
		fx.Provide(blockService),
		fx.Provide(peerRouting),
		fx.Provide(contentRouting),
		fx.Provide(addrsFactory(cfg.AnnounceAddresses, cfg.NoAnnounceAddresses)),
		fx.Provide(metrics.NewBandwidthCounter),
		fx.Provide(newModule),
		fx.Invoke(Listen(cfg.ListenAddresses)),
	)

	switch tp {
	case node.Full, node.Bridge:
		return fx.Module(
			"p2p",
			baseComponents,
			fx.Provide(blockstoreFromEDSStore),
			fx.Provide(func() (network.ResourceManager, error) {
				return rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.InfiniteLimits))
			}),
		)
	case node.Light:
		return fx.Module(
			"p2p",
			baseComponents,
			fx.Provide(blockstoreFromDatastore),
			fx.Provide(func(ctx context.Context, bootstrappers Bootstrappers) ([]ma.Multiaddr, error) {
				limits := rcmgr.DefaultLimits
				libp2p.SetDefaultServiceLimits(&limits)

				mutual, err := cfg.mutualPeers()
				if err != nil {
					return nil, err
				}

				// TODO(@Wondertan): We should resolve their addresses only once, but currently
				//  we resolve it here and libp2p stuck does that as well internally
				allowlist := make([]ma.Multiaddr, 0, len(bootstrappers)+len(mutual))
				for _, b := range bootstrappers {
					for _, baddr := range b.Addrs {
						resolved, err := madns.DefaultResolver.Resolve(ctx, baddr)
						if err != nil {
							log.Warnw("error resolving bootstrapper DNS", "addr", baddr.String(), "err", err)
							continue
						}
						allowlist = append(allowlist, resolved...)
					}
				}
				for _, m := range mutual {
					for _, maddr := range m.Addrs {
						resolved, err := madns.DefaultResolver.Resolve(ctx, maddr)
						if err != nil {
							log.Warnw("error resolving mutual peer DNS", "addr", maddr.String(), "err", err)
							continue
						}
						allowlist = append(allowlist, resolved...)
					}
				}

				return allowlist, nil
			}),
			fx.Provide(func(ctx context.Context, allowlist []ma.Multiaddr) (network.ResourceManager, error) {
				return rcmgr.NewResourceManager(
					rcmgr.NewFixedLimiter(rcmgr.DefaultLimits.AutoScale()),
					rcmgr.WithAllowlistedMultiaddrs(allowlist),
				)
			}),
		)
	default:
		panic("invalid node type")
	}
}
