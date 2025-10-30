package core

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

type exchangeMetrics struct {
	downloadDuration        metric.Float64Histogram
	edsConstructionDuration metric.Float64Histogram
	edsStorageDuration      metric.Float64Histogram
}

func newExchangeMetrics() (*exchangeMetrics, error) {
	m := new(exchangeMetrics)

	var err error
	m.downloadDuration, err = meter.Float64Histogram(
		"core_ex_block_download_time",
		metric.WithDescription("time to download block from core in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	m.edsConstructionDuration, err = meter.Float64Histogram(
		"core_ex_eds_construction_time",
		metric.WithDescription("time to construct EDS from block data in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	m.edsStorageDuration, err = meter.Float64Histogram(
		"core_ex_eds_storage_time",
		metric.WithDescription("time to store EDS in milliseconds"),
		metric.WithUnit("ms"),
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

func (m *exchangeMetrics) observeBlockDownload(ctx context.Context, duration time.Duration, edsSize int) {
	m.observe(ctx, func(ctx context.Context) {
		m.downloadDuration.Record(ctx, float64(duration.Milliseconds()),
			metric.WithAttributes(edsSizeAttribute(edsSize)))
	})
}

func (m *exchangeMetrics) observeEDSConstruction(ctx context.Context, duration time.Duration, edsSize int) {
	m.observe(ctx, func(ctx context.Context) {
		m.edsConstructionDuration.Record(ctx, float64(duration.Milliseconds()),
			metric.WithAttributes(edsSizeAttribute(edsSize)))
	})
}

func (m *exchangeMetrics) observeEDSStorage(ctx context.Context, duration time.Duration, edsSize int) {
	m.observe(ctx, func(ctx context.Context) {
		m.edsStorageDuration.Record(ctx, float64(duration.Milliseconds()),
			metric.WithAttributes(edsSizeAttribute(edsSize)))
	})
}

// edsSizeAttribute creates an attribute for the EDS square size
func edsSizeAttribute(size int) attribute.KeyValue {
	return attribute.Int("eds_size", size)
}
