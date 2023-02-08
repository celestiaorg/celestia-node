package sync

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

var meter = global.MeterProvider().Meter("header/sync")

type metrics struct {
	totalSynced int64
}

func (s *Syncer[H]) InitMetrics() error {
	s.metrics = &metrics{}

	totalSynced, err := meter.
		AsyncFloat64().
		Gauge(
			"total_synced_headers",
			instrument.WithDescription("total synced headers"),
		)
	if err != nil {
		return err
	}

	return meter.RegisterCallback(
		[]instrument.Asynchronous{
			totalSynced,
		},
		func(ctx context.Context) {
			totalSynced.Observe(ctx, float64(atomic.LoadInt64(&s.metrics.totalSynced)))
		},
	)
}

// recordTotalSynced records the total amount of synced headers.
func (m *metrics) recordTotalSynced(totalSynced int) {
	atomic.AddInt64(&m.totalSynced, int64(totalSynced))
}
