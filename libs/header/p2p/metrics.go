package p2p

import (
	"context"

	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
)

type metrics struct {
	responseSize     syncfloat64.Histogram
	responseDuration syncfloat64.Histogram
}

var (
	meter = global.MeterProvider().Meter("libs/header/p2p")
)

func (ex *Exchange[H]) RegisterMetrics() error {
	responseSize, err := meter.
		SyncFloat64().
		Histogram(
			"libhead_get_headers_response_size",
			instrument.WithDescription("Size of get headers response in bytes"),
		)
	if err != nil {
		panic(err)
	}

	responseDuration, err := meter.
		SyncFloat64().
		Histogram(
			"libhead_get_headers_request_duration",
			instrument.WithDescription("Duration of get headers request in seconds"),
		)
	if err != nil {
		return err
	}

	m := &metrics{
		responseSize:     responseSize,
		responseDuration: responseDuration,
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
