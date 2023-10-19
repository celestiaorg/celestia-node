package core

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("core")

var subscriptionStuckTimestampKey = "subscription_stuck_timestamp"

type metrics struct {
	blockTime     time.Time
	blockTimeInst metric.Float64Histogram

	subscriptionStuckInst metric.Int64Counter
}

func newMetrics() (*metrics, error) {
	m := new(metrics)

	var err error
	m.blockTimeInst, err = meter.Float64Histogram(
		"core_listener_block_time",
		metric.WithDescription("time between blocks coming through core listener block subscription"),
	)
	if err != nil {
		return nil, err
	}

	m.subscriptionStuckInst, err = meter.Int64Counter(
		"core_listener_subscription_stuck_count",
		metric.WithDescription("number of times core listener block subscription has been stuck/retried"),
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *metrics) observe(ctx context.Context, observeFn func(context.Context)) {
	if m == nil {
		return
	}

	if ctx.Err() != nil {
		ctx = context.Background()
	}

	observeFn(ctx)
}

func (m *metrics) observeBlockTime(ctx context.Context) {
	m.observe(ctx, func(ctx context.Context) {
		now := time.Now()

		if !m.blockTime.IsZero() {
			m.blockTimeInst.Record(ctx, now.Sub(m.blockTime).Seconds())
		}

		m.blockTime = now
	})
}

func (m *metrics) subscriptionStuck(ctx context.Context) {
	m.observe(ctx, func(ctx context.Context) {
		m.subscriptionStuckInst.Add(
			ctx,
			1,
			metric.WithAttributes(attribute.String(subscriptionStuckTimestampKey, time.Now().String())),
		)
	})
}
