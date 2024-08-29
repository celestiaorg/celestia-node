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
		fx.Invoke(enableBitswapMetrics),
	)
}

func libp2pMetrics(enable bool) libp2p.Option {
	if !enable {
		return libp2p.DisableMetrics()
	}
	registry := prometheus.DefaultRegisterer
	return libp2p.PrometheusRegisterer(registry)
}
