package core

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

var meter = otel.Meter("core")

type listenerMetrics struct {
	lastTimeSubscriptionStuck     time.Time
	lastTimeSubscriptionStuckInst metric.Int64ObservableGauge
	lastTimeSubscriptionStuckReg  metric.Registration

	subscriptionStuckInst metric.Int64Counter
}

func newListenerMetrics() (*listenerMetrics, error) {
	m := new(listenerMetrics)

	var err error
	m.subscriptionStuckInst, err = meter.Int64Counter(
		"core_listener_subscription_stuck_count",
		metric.WithDescription("number of times core listener block subscription has been stuck/retried"),
	)
	if err != nil {
		return nil, err
	}

	m.lastTimeSubscriptionStuckInst, err = meter.Int64ObservableGauge(
		"core_listener_last_time_subscription_stuck_timestamp",
		metric.WithDescription("last time the listener subscription was stuck"),
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

	return m, nil
}

func (m *listenerMetrics) observe(ctx context.Context, observeFn func(ctx context.Context)) {
	if m == nil {
		return
	}

	ctx = utils.ResetContextOnError(ctx)

	observeFn(ctx)
}

func (m *listenerMetrics) subscriptionStuck(ctx context.Context) {
	m.observe(ctx, func(ctx context.Context) {
		m.subscriptionStuckInst.Add(ctx, 1)
		m.lastTimeSubscriptionStuck = time.Now()
	})
}

func (m *listenerMetrics) observeLastTimeStuckCallback(_ context.Context, obs metric.Observer) error {
	obs.ObserveInt64(m.lastTimeSubscriptionStuckInst, m.lastTimeSubscriptionStuck.Unix())
	return nil
}

func (m *listenerMetrics) Close() error {
	if m == nil {
		return nil
	}

	return m.lastTimeSubscriptionStuckReg.Unregister()
}
