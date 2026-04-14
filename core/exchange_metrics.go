package core

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

type exchangeMetrics struct {
	totalBlocksProcessed metric.Int64Counter
}

func newExchangeMetrics() (*exchangeMetrics, error) {
	m := new(exchangeMetrics)

	var err error
	m.totalBlocksProcessed, err = meter.Int64Counter(
		"core_ex_total_blocks_processed",
		metric.WithDescription("total number of blocks processed by the exchange"),
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

	ctx = utils.ResetContextOnError(ctx)
	observeFn(ctx)
}

func (m *exchangeMetrics) observeBlockProcessed(ctx context.Context, edsSize int) {
	m.observe(ctx, func(ctx context.Context) {
		m.totalBlocksProcessed.Add(ctx, 1, metric.WithAttributes(edsSizeAttribute(edsSize)))
	})
}

// edsSizeAttribute creates an attribute for the EDS square size
func edsSizeAttribute(size int) attribute.KeyValue {
	return attribute.Int("eds_size", size)
}
