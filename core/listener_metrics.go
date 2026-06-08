package core

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

var meter = otel.Meter("core")

type listenerMetrics struct {
	lastTimeSubscriptionStuck     time.Time
	lastTimeSubscriptionStuckInst metric.Int64ObservableGauge
	lastTimeSubscriptionStuckReg  metric.Registration

	subscriptionStuckInst metric.Int64Counter

	lastProcessedBlockTs     time.Time
	lastProcessedBlockTsInst metric.Int64ObservableGauge
	lastProcessedBlockTsReg  metric.Registration

	coreToDaLagInst metric.Float64Histogram

	headerSubPublishedInst       metric.Int64Counter
	headerSubPublishDurationInst metric.Float64Histogram

	blockEventsInst metric.Int64Counter
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

	m.lastProcessedBlockTsInst, err = meter.Int64ObservableGauge(
		"core_listener_last_processed_block_ts",
		metric.WithDescription("unix timestamp (seconds) of the last block successfully processed and broadcast"),
	)
	if err != nil {
		return nil, err
	}
	m.lastProcessedBlockTsReg, err = meter.RegisterCallback(
		m.observeLastProcessedBlockTsCallback,
		m.lastProcessedBlockTsInst,
	)
	if err != nil {
		return nil, err
	}

	m.coreToDaLagInst, err = meter.Float64Histogram(
		"core_to_da_lag_seconds",
		metric.WithDescription("seconds between core block consensus time and DA broadcast"),
	)
	if err != nil {
		return nil, err
	}

	m.headerSubPublishedInst, err = meter.Int64Counter(
		"header_sub_published_total",
		metric.WithDescription("number of ExtendedHeaders published to HeaderSub by this bridge"),
	)
	if err != nil {
		return nil, err
	}

	m.headerSubPublishDurationInst, err = meter.Float64Histogram(
		"header_sub_publish_duration_seconds",
		metric.WithDescription("duration of HeaderSub Broadcast() call (seconds)"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	m.blockEventsInst, err = meter.Int64Counter(
		"core_block_events_total",
		metric.WithDescription(
			"new-block events from core sources, labeled by `source` (announcing endpoint) and "+
				"`result`: duplicate = skipped by store dedup; processed = stored & broadcast; "+
				"historic = dropped outside availability window; "+
				"fetch_error/sync_error/process_error/store_error = failed",
		),
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
	if m.lastTimeSubscriptionStuck.IsZero() {
		// Never stuck — don't emit a value, otherwise dashboards interpret
		// time.Time{}.Unix() as a timestamp ~2025 years in the past.
		return nil
	}
	obs.ObserveInt64(m.lastTimeSubscriptionStuckInst, m.lastTimeSubscriptionStuck.Unix())
	return nil
}

func (m *listenerMetrics) observeLastProcessedBlockTsCallback(_ context.Context, obs metric.Observer) error {
	if m.lastProcessedBlockTs.IsZero() {
		return nil
	}
	obs.ObserveInt64(m.lastProcessedBlockTsInst, m.lastProcessedBlockTs.Unix())
	return nil
}

// headerPublished records a HeaderSub Broadcast() outcome. `local` distinguishes
// catch-up broadcasts (delivered only to local subscribers, no network propagation)
// from steady-state network broadcasts. Errors are tagged with result=error.
func (m *listenerMetrics) headerPublished(ctx context.Context, dur time.Duration, local bool, err error) {
	m.observe(ctx, func(ctx context.Context) {
		result := "ok"
		switch {
		case errors.Is(err, context.Canceled):
			result = "canceled"
		case err != nil:
			result = "error"
		}
		attrs := metric.WithAttributes(
			attribute.String("result", result),
			attribute.Bool("local_only", local),
		)
		m.headerSubPublishedInst.Add(ctx, 1, attrs)
		m.headerSubPublishDurationInst.Record(ctx, dur.Seconds(), attrs)
	})
}

// blockProcessed records a successfully processed block: updates the last-processed
// timestamp and records the core->DA lag (consensus block time -> broadcast).
func (m *listenerMetrics) blockProcessed(ctx context.Context, blockTime time.Time) {
	m.observe(ctx, func(ctx context.Context) {
		m.lastProcessedBlockTs = time.Now()
		lag := time.Since(blockTime).Seconds()
		if lag < 0 {
			// clock skew between core and node; clamp rather than skip so the
			// metric remains continuous and visible in dashboards.
			lag = 0
		}
		m.coreToDaLagInst.Record(ctx, lag)
	})
}

// blockEvent records the terminal outcome of a single new-block event from a
// core source. Every BlockEvent funnels into exactly one `result`, so
// rate(processed) is throughput and rate(duplicate) quantifies the redundant
// downloads the fan-in dedup saved (≈ N-1 per height with N healthy sources).
// `source` is the announcing endpoint, so per-source rates expose which
// configured source actually contributes and which only ever loses the race.
func (m *listenerMetrics) blockEvent(ctx context.Context, source, result string) {
	m.observe(ctx, func(ctx context.Context) {
		m.blockEventsInst.Add(ctx, 1, metric.WithAttributes(
			attribute.String("source", source),
			attribute.String("result", result),
		))
	})
}

func (m *listenerMetrics) Close() error {
	if m == nil {
		return nil
	}

	return errors.Join(
		m.lastTimeSubscriptionStuckReg.Unregister(),
		m.lastProcessedBlockTsReg.Unregister(),
	)
}
