package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p/core/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
)

// global meter provider (see opentelemetry docs)
var (
	meter = global.MeterProvider().Meter("p2p")
)

// WithMetrics option sets up metrics for p2p networking.
func WithMetrics(bc *metrics.BandwidthCounter) {
	bandwidthTotalInbound, _ := meter.
		SyncInt64().
		Histogram(
			"p2p_bandwidth_total_inbound",
			instrument.WithUnit(unit.Bytes),
			instrument.WithDescription("total number of bytes received by the host"),
		)

	bandwidthTotalOutbound, _ := meter.
		SyncInt64().
		Histogram(
			"p2p_bandwidth_total_outbound",
			instrument.WithUnit(unit.Bytes),
			instrument.WithDescription("total number of bytes sent by the host"),
		)

	bandwidthRateInbound, _ := meter.
		SyncFloat64().
		Histogram(
			"p2p_bandwidth_rate_inbound",
			instrument.WithDescription("total number of bytes sent by the host"),
		)

	bandwidthRateOutbound, _ := meter.
		SyncFloat64().
		Histogram(
			"p2p_bandwidth_rate_outbound",
			instrument.WithDescription("total number of bytes sent by the host"),
		)

	bandwidthTotalInboundByPeer, _ := meter.
		SyncInt64().
		Histogram(
			"p2p_total_inbound_by_peer",
			instrument.WithDescription("total number of bytes received by the host by peer"),
		)

	bandwidthTotalOutboundByPeer, _ := meter.
		SyncInt64().
		Histogram(
			"p2p_total_outbound_by_peer",
			instrument.WithDescription("total number of bytes sent by the host by peer"),
		)

	bandwidthInboundRateByPeer, _ := meter.
		SyncFloat64().
		Histogram(
			"p2p_rate_inbound_by_peer",
			instrument.WithDescription("rate of bytes received by the host by peer"),
		)

	bandwidthOutboundRateByPeer, _ := meter.
		SyncFloat64().
		Histogram(
			"p2p_rate_outbound_by_peer",
			instrument.WithDescription("rate of bytes sent by the host by peer"),
		)

	err := meter.RegisterCallback(
		[]instrument.Asynchronous{}, func(ctx context.Context) {
			bcStats := bc.GetBandwidthTotals()
			bcByPeerStats := bc.GetBandwidthByPeer()

			bandwidthTotalInbound.Record(ctx, bcStats.TotalIn)
			bandwidthTotalOutbound.Record(ctx, bcStats.TotalOut)
			bandwidthRateInbound.Record(ctx, bcStats.RateIn)
			bandwidthRateOutbound.Record(ctx, bcStats.RateOut)

			for peerID, stat := range bcByPeerStats {
				bandwidthTotalInboundByPeer.Record(
					ctx,
					stat.TotalIn,
					attribute.String("peer_id", peerID.Pretty()),
				)

				bandwidthTotalOutboundByPeer.Record(
					ctx,
					stat.TotalOut,
					attribute.String("peer_id", peerID.Pretty()),
				)

				bandwidthInboundRateByPeer.Record(
					ctx,
					stat.RateIn,
					attribute.String("peer_id", peerID.Pretty()),
				)

				bandwidthOutboundRateByPeer.Record(
					ctx,
					stat.RateOut,
					attribute.String("peer_id", peerID.Pretty()),
				)
			}
		},
	)

	if err != nil {
		panic(err)
	}
}
