// This file defines metrics relative to the nodebuilder package.
package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
)

type metrics struct {
	// nodeStartTS is the timestamp when the node was started.
	nodeStartTS syncfloat64.UpDownCounter

	// totalNodeUptime is the total time the node has been running.
	totalNodeUptime syncfloat64.Counter

	// lastNodeUptimeTs is the last timestamp when the node uptime was recorded.
	lastNodeUptimeTs float64
}

var (
	meter = global.MeterProvider().Meter("node")
)

func newNodeMetrics() (*metrics, error) {
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

	return &metrics{
		nodeStartTS:     nodeStartTS,
		totalNodeUptime: totalNodeUptime,
	}, nil
}

// recordNodeStart records the timestamp when the node was started.
func (m *metrics) RecordNodeStartTime(ctx context.Context) {
	m.nodeStartTS.Add(context.Background(), float64(time.Now().Unix()))
}

// recordNodeUptime records the total time the node has been running.
func (m *metrics) ObserveNodeUptime(ctx context.Context, interval time.Duration) {
	m.lastNodeUptimeTs = float64(time.Now().Unix())

	// ticker ticks every `interval` and records the total time the node has been running
	// since the last tick
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ts := time.Since(time.Unix(int64(m.lastNodeUptimeTs), 0)).Seconds()
			m.lastNodeUptimeTs = ts
			m.totalNodeUptime.Add(context.Background(), ts)
		case <-ctx.Done():
			return
		}
	}
}
