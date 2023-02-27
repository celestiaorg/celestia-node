package p2p

import (
	"context"
	"net/http"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/libp2p/go-libp2p/core/metrics"
	rcmgrObs "github.com/libp2p/go-libp2p/p2p/host/resource-manager/obs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
	"go.uber.org/fx"

	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	ma "github.com/multiformats/go-multiaddr"
)

// global meter provider (see opentelemetry docs)
var (
	meter = global.MeterProvider().Meter("p2p")
)

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
func WithDebugMetrics(lifecycle fx.Lifecycle, cfg Config) error {
	rcmgrObs.MustRegisterWith(prometheus.DefaultRegisterer)

	reg := prometheus.DefaultRegisterer
	registry := reg.(*prometheus.Registry)

	mux := http.NewServeMux()
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: reg})
	mux.Handle(cfg.PrometheusAgentEndpoint, handler)

	promHttpServer := &http.Server{
		Addr:    cfg.PrometheusAgentPort,
		Handler: mux,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := promHttpServer.ListenAndServe(); err != nil {
					log.Error("Error starting Prometheus metrics exporter http server")
					panic(err)
				}
			}()

			log.Info("Prometheus agent started on %s/%s", cfg.PrometheusAgentPort, cfg.PrometheusAgentEndpoint)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := promHttpServer.Shutdown(ctx); err != nil {
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
