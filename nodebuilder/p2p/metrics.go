package p2p

import (
	"context"
	"net/http"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	rcmgrObs "github.com/libp2p/go-libp2p/p2p/host/resource-manager/obs"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// WithDebugMetrics option sets up metrics for p2p networking.
func WithLibp2pMetrics(lifecycle fx.Lifecycle, cfg Config) error {
	rcmgrObs.MustRegisterWith(prometheus.DefaultRegisterer)

	reg := prometheus.DefaultRegisterer
	registry := reg.(*prometheus.Registry)

	mux := http.NewServeMux()
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: reg})
	mux.Handle(cfg.PrometheusAgentEndpoint, handler)

	promHTTPServer := &http.Server{
		Addr:              cfg.PrometheusAgentPort,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := promHTTPServer.ListenAndServe(); err != nil {
					log.Error("Error starting Prometheus metrics exporter http server")
					panic(err)
				}
			}()

			log.Info("Prometheus agent started on %s/%s", cfg.PrometheusAgentPort, cfg.PrometheusAgentEndpoint)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := promHTTPServer.Shutdown(ctx); err != nil {
				return err
			}
			return nil
		},
	})

	return nil
}

func WithMonitoredResourceManager(nodeType node.Type, allowlist []ma.Multiaddr) (network.ResourceManager, error) {
	str, err := rcmgrObs.NewStatsTraceReporter()
	if err != nil {
		return nil, err
	}

	var (
		monitoredRcmgr network.ResourceManager
	)

	switch nodeType {
	case node.Full, node.Bridge:
		monitoredRcmgr, err = rcmgr.NewResourceManager(
			rcmgr.NewFixedLimiter(rcmgr.InfiniteLimits),
			rcmgr.WithTraceReporter(str),
		)

	case node.Light:
		monitoredRcmgr, err = rcmgr.NewResourceManager(
			rcmgr.NewFixedLimiter(rcmgr.DefaultLimits.AutoScale()),
			rcmgr.WithAllowlistedMultiaddrs(allowlist),
			rcmgr.WithTraceReporter(str),
		)
	default:
		panic("invalid node type")
	}

	if err != nil {
		return nil, err
	}

	return monitoredRcmgr, nil
}
