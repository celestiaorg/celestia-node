// This file defines UptimeMetrics relative to the nodebuilder package.
package node

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
)

// UptimeMetrics is a struct that records
// 1. node start time: the timestamp when the node was started
// 2. node up time: total time the node has been running
//
// the node start time is recorded imperatively when RecordNodeStartTime is called
// whereas the node up time is recorded periodically upon callback recalling (re-mettering from optl)
type UptimeMetrics struct {
	// nodeStartTS is the timestamp when the node was started.
	nodeStartTS syncfloat64.UpDownCounter

	// totalNodeUptime is the total time the node has been running.
	totalNodeUptime asyncfloat64.Gauge

	// lastNodeUptimeTS is the last timestamp when the node uptime was recorded.
	lastNodeUptimeTS float64
}

var meter = global.MeterProvider().Meter("node")

// NewUptimeMetrics creates a new UptimeMetrics
// and registers a callback to re-meter the totalNodeUptime metric.
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
		AsyncFloat64().
		Gauge(
			"node_uptime",
			instrument.WithDescription("total time the node has been running"),
		)
	if err != nil {
		return nil, err
	}

	m := &UptimeMetrics{
		nodeStartTS:     nodeStartTS,
		totalNodeUptime: totalNodeUptime,
	}

	m.lastNodeUptimeTS = float64(time.Now().Unix())

	err = meter.RegisterCallback(
		[]instrument.Asynchronous{
			totalNodeUptime,
		},
		func(ctx context.Context) {
			ts := time.Since(time.Unix(int64(m.lastNodeUptimeTS), 0)).Seconds()
			m.lastNodeUptimeTS = float64(time.Now().Unix())
			totalNodeUptime.Observe(ctx, ts)
		},
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// RecordNodeStartTime records the timestamp when the node was started.
func (m *UptimeMetrics) RecordNodeStartTime(ctx context.Context) {
	m.nodeStartTS.Add(context.Background(), float64(time.Now().Unix()))
}
