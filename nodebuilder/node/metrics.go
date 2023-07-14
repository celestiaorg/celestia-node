package node

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("node")

var (
	timeStarted time.Time
	nodeStarted bool
)

// WithMetrics registers node metrics.
func WithMetrics() error {
	nodeStartTS, err := meter.Int64ObservableGauge(
		"node_start_ts",
		metric.WithDescription("timestamp when the node was started"),
	)
	if err != nil {
		return err
	}

	totalNodeRunTime, err := meter.Float64ObservableCounter(
		"node_runtime_counter_in_seconds",
		metric.WithDescription("total time the node has been running"),
	)
	if err != nil {
		return err
	}

	callback := func(ctx context.Context, observer metric.Observer) error {
		if !nodeStarted {
			// Observe node start timestamp
			timeStarted = time.Now()
			observer.ObserveInt64(nodeStartTS, timeStarted.Unix())
			nodeStarted = true
		}

		observer.ObserveFloat64(totalNodeRunTime, time.Since(timeStarted).Seconds())
		return nil
	}

	_, err = meter.RegisterCallback(callback, nodeStartTS, totalNodeRunTime)
	return err
}
