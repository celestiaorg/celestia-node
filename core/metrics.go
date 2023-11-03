package core

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("core")

type metrics struct {
	lastTimeSubscriptionStuck     time.Time
	lastTimeSubscriptionStuckInst metric.Int64Observable
	lastTimeSubscriptionStuckReg  metric.Registration

	subscriptionStuckInst metric.Int64Counter

	getByHeightDuration metric.Float64Histogram
}

func newMetrics() (*metrics, error) {
	m := new(metrics)

	var err error
	m.subscriptionStuckInst, err = meter.Int64Counter(
		"core_listener_subscription_stuck_count",
		metric.WithDescription("number of times core listener block subscription has been stuck/retried"),
	)
	if err != nil {
		return nil, err
	}

	m.lastTimeSubscriptionStuckReg, err = meter.RegisterCallback(
		m.observeLastTimeStuckCallback,
		m.lastTimeSubscriptionStuckInst,
	)
	if err != nil {
		return nil, err
	}

	m.getByHeightDuration, err = meter.Float64Histogram(
		"core_ex_get_by_height_request_time",
		metric.WithDescription("core exchange client getByHeight request time in seconds (per single height)"),
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

func (m *metrics) subscriptionStuck(ctx context.Context) {
	m.observe(ctx, func(ctx context.Context) {
		m.subscriptionStuckInst.Add(ctx, 1)
		m.lastTimeSubscriptionStuck = time.Now()
	})
}

func (m *metrics) observeLastTimeStuckCallback(_ context.Context, obs metric.Observer) error {
	obs.ObserveInt64(m.lastTimeSubscriptionStuckInst, m.lastTimeSubscriptionStuck.Unix())
	return nil
}

func (m *metrics) requestDurationPerHeader(ctx context.Context, duration time.Duration, amount uint64) {
	m.observe(ctx, func(ctx context.Context) {
		if amount == 0 {
			// TODO could this happen?
			return
		}
		durationPerHeader := duration.Seconds() / float64(amount)
		m.getByHeightDuration.Record(ctx, durationPerHeader)
	})
}

func (m *metrics) Close() error {
	if m == nil {
		return nil
	}

	return m.lastTimeSubscriptionStuckReg.Unregister()
}
