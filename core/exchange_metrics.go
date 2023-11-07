package core

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/metric"
)

type exchangeMetrics struct {
	getByHeightDuration metric.Float64Histogram // TODO move this one
}

func newExchangeMetrics() (*exchangeMetrics, error) {
	m := new(exchangeMetrics)

	var err error
	m.getByHeightDuration, err = meter.Float64Histogram(
		"core_ex_get_by_height_request_time",
		metric.WithDescription("core exchange client getByHeight request time in seconds (per single height)"),
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *exchangeMetrics) observe(ctx context.Context, observeFn func(ctx context.Context)) {
	if m == nil {
		return
	}

	if ctx.Err() != nil {
		ctx = context.Background()
	}

	observeFn(ctx)
}

func (m *exchangeMetrics) requestDurationPerHeader(ctx context.Context, duration time.Duration, amount uint64) {
	m.observe(ctx, func(ctx context.Context) {
		if amount == 0 {
			// TODO could this happen?
			return
		}
		durationPerHeader := duration.Seconds() / float64(amount)
		m.getByHeightDuration.Record(ctx, durationPerHeader)
	})
}
