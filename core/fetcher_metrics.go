package core

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

type fetcherMetrics struct {
	receiveBlockDuration metric.Float64Histogram
}

func newFetcherMetrics() (*fetcherMetrics, error) {
	m := new(fetcherMetrics)

	var err error
	m.receiveBlockDuration, err = meter.Float64Histogram(
		"core_fetcher_receive_block_time",
		metric.WithDescription("time to receive block from core in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *fetcherMetrics) observe(ctx context.Context, observeFn func(ctx context.Context)) {
	if m == nil {
		return
	}

	ctx = utils.ResetContextOnError(ctx)
	observeFn(ctx)
}

func (m *fetcherMetrics) observeReceiveBlock(ctx context.Context, duration time.Duration, numParts int) {
	m.observe(ctx, func(ctx context.Context) {
		m.receiveBlockDuration.Record(ctx, float64(duration.Milliseconds()),
			// can be used to approximate the block size (with each part being roughly
			// 64KB)
			metric.WithAttributes(attribute.Int("num_parts", numParts)))
	})
}
