package node

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

var meter = global.MeterProvider().Meter("node")

var (
	timeStarted time.Time
	nodeStarted bool
)

// WithMetrics registers node metrics.
func WithMetrics() error {
	nodeStartTS, err := meter.
		AsyncFloat64().
		Gauge(
			"node_start_ts",
			instrument.WithDescription("timestamp when the node was started"),
		)
	if err != nil {
		return err
	}

	totalNodeRunTime, err := meter.
		AsyncFloat64().
		Counter(
			"node_runtime_counter_in_seconds",
			instrument.WithDescription("total time the node has been running"),
		)
	if err != nil {
		return err
	}

	return meter.RegisterCallback(
		[]instrument.Asynchronous{nodeStartTS, totalNodeRunTime},
		func(ctx context.Context) {
			if !nodeStarted {
				// Observe node start timestamp
				timeStarted = time.Now()
				nodeStartTS.Observe(ctx, float64(timeStarted.Unix()))
				nodeStarted = true
			}

			totalNodeRunTime.Observe(ctx, time.Since(timeStarted).Seconds())
		},
	)
}
