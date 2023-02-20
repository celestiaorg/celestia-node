package p2p

import (
	"context"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
)

type metrics struct {
	responseSize     syncfloat64.Histogram
	responseDuration syncfloat64.Histogram
	bestHead         asyncfloat64.Gauge
	blockTime        syncfloat64.Histogram

	latestHead int64
	lastHeadTS int64
}

var (
	meter = global.MeterProvider().Meter("libs/header/p2p")
)

func (ex *Exchange[H]) RegisterMetrics() error {
	responseSize, err := meter.
		SyncFloat64().
		Histogram(
			"get_headers_response_size",
			instrument.WithDescription("Size of get headers response in bytes"),
		)
	if err != nil {
		panic(err)
	}

	responseDuration, err := meter.
		SyncFloat64().
		Histogram(
			"get_headers_request_duration",
			instrument.WithDescription("Duration of get headers request in seconds"),
		)
	if err != nil {
		return err
	}

	bestHead, err := meter.
		AsyncFloat64().
		Gauge(
			"best_head",
			instrument.WithDescription("Latest/best observed height"),
		)
	if err != nil {
		return err
	}

	blockTime, err := meter.
		SyncFloat64().
		Histogram(
			"node.header.blackbox.block_time",
			instrument.WithDescription("block time"),
		)
	if err != nil {
		return err
	}

	m := &metrics{
		responseSize:     responseSize,
		responseDuration: responseDuration,
		blockTime:        blockTime,
		latestHead:       0,
		lastHeadTS:       0,
	}

	err = meter.RegisterCallback([]instrument.Asynchronous{bestHead}, func(ctx context.Context) {
		head := atomic.LoadInt64(&m.latestHead)
		bestHead.Observe(ctx, float64(head))
	})
	if err != nil {
		return err
	}

	ex.metrics = m

	return nil
}

func (m *metrics) ObserveRequest(ctx context.Context, size uint64, duration uint64) {
	if m == nil {
		return
	}
	m.responseSize.Record(ctx, float64(size))
	m.responseDuration.Record(ctx, float64(duration))
}

func (m *metrics) ObserveBestHead(ctx context.Context, head int64) {
	if m == nil {
		return
	}

	atomic.StoreInt64(&m.latestHead, head)

	if lastHeight := atomic.LoadInt64(&m.latestHead); head <= lastHeight {
		return
	}

	lastHeadTS := atomic.LoadInt64(&m.lastHeadTS)
	now := time.Now().Unix()

	m.blockTime.Record(
		ctx,
		time.Duration(now-lastHeadTS).Seconds(),
		attribute.Int("height", int(head)),
	)

	atomic.StoreInt64(&m.latestHead, now)
}
