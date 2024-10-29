// network.go
package system

import (
	"context"
	"fmt"
	"github.com/shirou/gopsutil/v3/net"
	"go.opentelemetry.io/otel/metric"
)

type NetworkMetrics struct {
	bytesIn    metric.Float64ObservableGauge
	bytesOut   metric.Float64ObservableGauge
	packetsIn  metric.Float64ObservableGauge
	packetsOut metric.Float64ObservableGauge
	errorsIn   metric.Float64ObservableGauge
	errorsOut  metric.Float64ObservableGauge
}

func newNetworkMetrics() (*NetworkMetrics, error) {
	bytesIn, err := meter.Float64ObservableGauge("system_network_receive_bytes",
		metric.WithDescription("Network bytes received"),
		metric.WithUnit("bytes"))
	if err != nil {
		return nil, fmt.Errorf("network bytes in metric: %w", err)
	}

	bytesOut, err := meter.Float64ObservableGauge("system_network_transmit_bytes",
		metric.WithDescription("Network bytes transmitted"),
		metric.WithUnit("bytes"))
	if err != nil {
		return nil, fmt.Errorf("network bytes out metric: %w", err)
	}

	packetsIn, err := meter.Float64ObservableGauge("system_network_receive_packets",
		metric.WithDescription("Network packets received"),
		metric.WithUnit("packets"))
	if err != nil {
		return nil, fmt.Errorf("network packets in metric: %w", err)
	}

	packetsOut, err := meter.Float64ObservableGauge("system_network_transmit_packets",
		metric.WithDescription("Network packets transmitted"),
		metric.WithUnit("packets"))
	if err != nil {
		return nil, fmt.Errorf("network packets out metric: %w", err)
	}

	errorsIn, err := meter.Float64ObservableGauge("system_network_receive_errors",
		metric.WithDescription("Network receive errors"),
		metric.WithUnit("errors"))
	if err != nil {
		return nil, fmt.Errorf("network errors in metric: %w", err)
	}

	errorsOut, err := meter.Float64ObservableGauge("system_network_transmit_errors",
		metric.WithDescription("Network transmit errors"),
		metric.WithUnit("errors"))
	if err != nil {
		return nil, fmt.Errorf("network errors out metric: %w", err)
	}

	return &NetworkMetrics{
		bytesIn:    bytesIn,
		bytesOut:   bytesOut,
		packetsIn:  packetsIn,
		packetsOut: packetsOut,
		errorsIn:   errorsIn,
		errorsOut:  errorsOut,
	}, nil
}

func (n *NetworkMetrics) Metrics() []metric.Observable {
	return []metric.Observable{
		n.bytesIn,
		n.bytesOut,
		n.packetsIn,
		n.packetsOut,
		n.errorsIn,
		n.errorsOut,
	}
}

func (n *NetworkMetrics) Collect(ctx context.Context, observer metric.Observer) error {
	stats, err := net.IOCounters(false)
	if err != nil {
		return fmt.Errorf("get network stats: %w", err)
	}

	if len(stats) > 0 {
		stat := stats[0]
		observer.ObserveFloat64(n.bytesIn, float64(stat.BytesRecv))
		observer.ObserveFloat64(n.bytesOut, float64(stat.BytesSent))
		observer.ObserveFloat64(n.packetsIn, float64(stat.PacketsRecv))
		observer.ObserveFloat64(n.packetsOut, float64(stat.PacketsSent))
		observer.ObserveFloat64(n.errorsIn, float64(stat.Errin))
		observer.ObserveFloat64(n.errorsOut, float64(stat.Errout))
	}

	return nil
}
