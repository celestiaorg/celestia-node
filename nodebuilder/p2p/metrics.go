package p2p

import (
	"context"
	"fmt"
	"net/http"

	"github.com/libp2p/go-libp2p/core/metrics"
	rcmgrObs "github.com/libp2p/go-libp2p/p2p/host/resource-manager/obs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
	"go.uber.org/fx"
)

// global meter provider (see opentelemetry docs)
var (
	meter = global.MeterProvider().Meter("p2p")
)

// newPromAgentHttpServer configures and return an http server
// that handles prometheus requests
func newPromAgentHttpServer(cfg Config, reg prometheus.Registerer) *http.Server {
	registry := reg.(*prometheus.Registry)

	mux := http.NewServeMux()
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: reg})
	mux.Handle(cfg.PrometheusAgentEndpoint, handler)

	return &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.PrometheusAgentPort),
		Handler: mux,
	}
}

// WithPrometheusAgentOpts is a funciton that provides an instance
// of an http server to be used with prometheus agent.
// Use with fx.Provide, only necessary when `WithDebugMetrics` is used.
func WithPrometheusAgentOpts() fx.Option {
	return fx.Options(
		fx.Provide(
			func() prometheus.Registerer {
				return prometheus.DefaultRegisterer
			},
		),
		fx.Provide(
			fx.Annotate(
				newPromAgentHttpServer,
				fx.OnStart(func(startCtx, ctx context.Context, promHttpServer *http.Server) error {
					go func() {
						if err := promHttpServer.ListenAndServe(); err != nil {
							log.Error("Error starting Prometheus metrics exporter http server")
							panic(err)
						}
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
}

// WithInfoMetrics option sets up metrics for p2p networking.
func WithInfoMetrics(bc *metrics.BandwidthCounter) error {
	bandwidthTotalInbound, err := meter.
		SyncInt64().
		Histogram(
			"p2p_bandwidth_total_inbound",
			instrument.WithUnit(unit.Bytes),
			instrument.WithDescription("total number of bytes received by the host"),
		)
	if err != nil {
		return err
	}
	bandwidthTotalOutbound, _ := meter.
		SyncInt64().
		Histogram(
			"p2p_bandwidth_total_outbound",
			instrument.WithUnit(unit.Bytes),
			instrument.WithDescription("total number of bytes sent by the host"),
		)
	if err != nil {
		return err
	}
	bandwidthRateInbound, _ := meter.
		SyncFloat64().
		Histogram(
			"p2p_bandwidth_rate_inbound",
			instrument.WithDescription("total number of bytes sent by the host"),
		)
	if err != nil {
		return err
	}
	bandwidthRateOutbound, _ := meter.
		SyncFloat64().
		Histogram(
			"p2p_bandwidth_rate_outbound",
			instrument.WithDescription("total number of bytes sent by the host"),
		)
	if err != nil {
		return err
	}
	p2pPeerCount, _ := meter.
		AsyncFloat64().
		Gauge(
			"p2p_peer_count",
			instrument.WithDescription("number of peers connected to the host"),
		)
	if err != nil {
		return err
	}

	if err = meter.RegisterCallback(
		[]instrument.Asynchronous{
			p2pPeerCount,
		}, func(ctx context.Context) {
			bcStats := bc.GetBandwidthTotals()
			bcByPeerStats := bc.GetBandwidthByPeer()

			bandwidthTotalInbound.Record(ctx, bcStats.TotalIn)
			bandwidthTotalOutbound.Record(ctx, bcStats.TotalOut)
			bandwidthRateInbound.Record(ctx, bcStats.RateIn)
			bandwidthRateOutbound.Record(ctx, bcStats.RateOut)

			p2pPeerCount.Observe(ctx, float64(len(bcByPeerStats)))
		},
	); err != nil {
		return err
	}

	return nil
}

// WithDebugMetrics option sets up metrics for p2p networking.
func WithDebugMetrics(promRegisterer prometheus.Registerer) error {
	rcmgrObs.MustRegisterWith(promRegisterer)
	return nil
}
