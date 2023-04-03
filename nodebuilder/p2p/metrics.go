package p2p

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	rcmgrObs "github.com/libp2p/go-libp2p/p2p/host/resource-manager/obs"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

const promAgentEndpoint = "/metrics"

type MetricsConfig struct {
	// Prometheus Agent http configuration
	PrometheusAgentPort string
}

// DefaultMetricsConfig returns default configuration for P2P subsystem.
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		PrometheusAgentPort: "8890",
	}
}

func (cfg *MetricsConfig) Validate() error {
	if cfg.PrometheusAgentPort == "" {
		return fmt.Errorf("p2p metrics: prometheus agent port cannot be empty")
	}

	pattern := "^\\d+$"
	regex := regexp.MustCompile(pattern)
	if !regex.MatchString(cfg.PrometheusAgentPort) {
		return fmt.Errorf("p2p metrics: prometheus agent port must be a number")
	}

	return nil
}

// WithLibp2pMetrics option sets up native libp2p metrics up
func WithLibp2pMetrics(lifecycle fx.Lifecycle, cfg Config) error {
	rcmgrObs.MustRegisterWith(prometheus.DefaultRegisterer)

	reg := prometheus.DefaultRegisterer
	registry := reg.(*prometheus.Registry)

	mux := http.NewServeMux()
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: reg})
	mux.Handle(promAgentEndpoint, handler)

	promHTTPServer := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Metrics.PrometheusAgentPort),
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

			log.Info("Prometheus agent started on :%s/%s", cfg.Metrics.PrometheusAgentPort, promAgentEndpoint)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return promHTTPServer.Shutdown(ctx)
		},
	})
	return nil
}

func WithMonitoredResourceManager(nodeType node.Type, allowlist []ma.Multiaddr) (network.ResourceManager, error) {
	str, err := rcmgrObs.NewStatsTraceReporter()
	if err != nil {
		return nil, err
	}

	var monitoredRcmgr network.ResourceManager

	switch nodeType {
	case node.Full, node.Bridge:
		monitoredRcmgr, err = rcmgr.NewResourceManager(
			rcmgr.NewFixedLimiter(rcmgr.InfiniteLimits),
			rcmgr.WithTraceReporter(str),
		)

	case node.Light:
		limits := rcmgr.DefaultLimits
		libp2p.SetDefaultServiceLimits(&limits)

		monitoredRcmgr, err = rcmgr.NewResourceManager(
			rcmgr.NewFixedLimiter(limits.AutoScale()),
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
