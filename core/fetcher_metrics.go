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
			metric.WithAttributes(blockSizeAttribute(numParts)))
	})
}

// blockSizeAttribute creates an attribute for the block size in MB
// Each part is roughly 64KB, so numParts * 64KB / 1024 = numParts * 0.0625 MB
// NOTE: this is an approximation and not 100% accurate.
func blockSizeAttribute(numParts int) attribute.KeyValue {
	sizeInMB := float64(numParts) * 64.0 / 1024.0
	return attribute.Float64("approx_block_size_mb", sizeInMB)
}
