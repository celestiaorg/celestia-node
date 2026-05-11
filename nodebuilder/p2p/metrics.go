package p2p

import (
	"github.com/libp2p/go-libp2p"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"
)

// WithMetrics option sets up native libp2p metrics up.
func WithMetrics() fx.Option {
	return fx.Options(
		fx.Provide(resourceManagerOpt(traceReporter)),
		fx.Provide(prometheusRegisterer),
	)
}

// prometheusRegisterer provides a labeled prometheus.Registerer for libp2p and bitswap metrics.
// The registerer is also set as the global default so that bitswap metrics (which register
// with prometheus.DefaultRegisterer) are captured with the correct labels.
// All collected prometheus metrics are bridged to OTel via the prometheus bridge in
// nodebuilder/settings.go, eliminating the need for a standalone Prometheus HTTP server.
func prometheusRegisterer() prometheus.Registerer {
	return prometheus.DefaultRegisterer
}

// libp2pMetricsOpt returns a libp2p option that either enables or disables prometheus metrics.
func libp2pMetricsOpt(reg prometheus.Registerer) libp2p.Option {
	if reg != nil {
		return libp2p.PrometheusRegisterer(reg)
	}
	return libp2p.DisableMetrics()
}
