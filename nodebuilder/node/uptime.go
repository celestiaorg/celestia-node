// This file defines UptimeMetrics relative to the nodebuilder package.
package node

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
)

// TODO @derrandz doc this
type UptimeMetrics struct {
	// nodeStartTS is the timestamp when the node was started.
	nodeStartTS syncfloat64.UpDownCounter

	// totalNodeUptime is the total time the node has been running.
	totalNodeUptime syncfloat64.Counter

	// lastNodeUptimeTS is the last timestamp when the node uptime was recorded.
	lastNodeUptimeTS float64
}

var meter = global.MeterProvider().Meter("node")

func NewUptimeMetrics() (*UptimeMetrics, error) {
	nodeStartTS, err := meter.
		SyncFloat64().
		UpDownCounter(
			"node_start_ts",
			instrument.WithDescription("timestamp when the node was started"),
		)
	if err != nil {
		return nil, err
	}

	totalNodeUptime, err := meter.
		SyncFloat64().
		Counter(
			"node_uptime",
			instrument.WithDescription("total time the node has been running"),
		)
	if err != nil {
		return nil, err
	}

	return &UptimeMetrics{
		nodeStartTS:     nodeStartTS,
		totalNodeUptime: totalNodeUptime,
	}, nil
}

// RecordNodeStartTime records the timestamp when the node was started.
func (m *UptimeMetrics) RecordNodeStartTime(ctx context.Context) {
	m.nodeStartTS.Add(context.Background(), float64(time.Now().Unix()))
}

// ObserveNodeUptime records the total time the node has been running.
func (m *UptimeMetrics) ObserveNodeUptime(ctx context.Context, interval time.Duration) {
	m.lastNodeUptimeTS = float64(time.Now().Unix())

	// ticker ticks every `interval` and records the total time the node has been running
	// since the last tick
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ts := time.Since(time.Unix(int64(m.lastNodeUptimeTS), 0)).Seconds()
			m.lastNodeUptimeTS = float64(time.Now().Unix())
			m.totalNodeUptime.Add(ctx, ts)
		case <-ctx.Done():
			return
		}
	}
}
