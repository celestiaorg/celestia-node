package node

import (
	"context"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/fx"
)

var log = logging.Logger("module/node")

var meter = otel.Meter("node")

var (
	timeStarted time.Time
	nodeStarted bool
)

// WithMetrics registers node metrics.
func WithMetrics(lc fx.Lifecycle) error {
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

	buildInfoGauge, err := meter.Float64ObservableGauge(
		"build_info",
		metric.WithDescription("Celestia Node build information"),
	)
	if err != nil {
		return err
	}

	callback := func(_ context.Context, observer metric.Observer) error {
		if !nodeStarted {
			// Observe node start timestamp
			timeStarted = time.Now()
			observer.ObserveInt64(nodeStartTS, timeStarted.Unix())
			nodeStarted = true
		}

		observer.ObserveFloat64(totalNodeRunTime, time.Since(timeStarted).Seconds())

		// Observe build info with labels
		labels := metric.WithAttributes(
			attribute.String("build_time", buildTime),
			attribute.String("last_commit", lastCommit),
			attribute.String("semantic_version", semanticVersion),
			attribute.String("system_version", systemVersion),
			attribute.String("golang_version", golangVersion),
		)

		observer.ObserveFloat64(buildInfoGauge, 1, labels)

		return nil
	}

	clientReg, err := meter.RegisterCallback(callback, nodeStartTS, totalNodeRunTime, buildInfoGauge)
	if err != nil {
		return fmt.Errorf("failed to register metrics callback: %w", err)
	}

	lc.Append(
		fx.Hook{OnStop: func(context.Context) error {
			if err := clientReg.Unregister(); err != nil {
				log.Warn("failed to close metrics", "err", err)
			}
			return nil
		}},
	)
	return nil
}
