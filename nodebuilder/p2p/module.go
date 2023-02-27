package p2p

import (
	"context"
	"fmt"
	"net/http"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	rcmgrObs "github.com/libp2p/go-libp2p/p2p/host/resource-manager/obs"
	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	if cfg.EnableDebugMetrics {
		log.Info("Debug metrics enabled")
		baseComponents = fx.Options(
			baseComponents,
			fx.Provide(
				func() prometheus.Registerer {
					return prometheus.DefaultRegisterer
				},
			),
			fx.Invoke(
				func(promRegisterer prometheus.Registerer) error {
					rcmgrObs.MustRegisterWith(promRegisterer)
					return nil
				},
			),
			fx.Provide(
				fx.Annotate(
					func(cfg Config, reg prometheus.Registerer) *http.Server {
						registry := reg.(*prometheus.Registry)

						mux := http.NewServeMux()
						handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: reg})
						mux.Handle(cfg.PrometheusAgentEndpoint, handler)

						return &http.Server{
							Addr:    fmt.Sprintf(":%s", cfg.PrometheusAgentPort),
							Handler: mux,
						}
					},
					fx.OnStart(func(startCtx, ctx context.Context, promHttpServer *http.Server) error {
						log.Info("Starting prometheus agent")
						go func() {
							if err := promHttpServer.ListenAndServe(); err != nil {
								log.Error("Error starting Prometheus metrics exporter http server")
								panic(err)
							}
							log.Info("Prometheus agent started on %s/%s", cfg.PrometheusAgentPort, cfg.PrometheusAgentEndpoint)
						}()
						return nil
					}),
					fx.OnStop(func(ctx context.Context, promHttpServer *http.Server) error {
						if err := promHttpServer.Shutdown(ctx); err != nil {
							return err
						}
						return nil
					}),
				),
			),
		)
	} else {
		log.Info("Debug metrics disabled")
	}

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
			fx.Provide(func(ctx context.Context, bootstrappers Bootstrappers) (network.ResourceManager, error) {
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

				return rcmgr.NewResourceManager(
					rcmgr.NewFixedLimiter(limits.AutoScale()),
					rcmgr.WithAllowlistedMultiaddrs(allowlist),
				)
			}),
		)
	default:
		panic("invalid node type")
	}
}
