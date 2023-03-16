package p2p

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
)

type metrics struct {
	responseSize     syncfloat64.Histogram
	responseDuration syncfloat64.Histogram
}

var (
	meter = global.MeterProvider().Meter("header/p2p")
)

func (ex *Exchange[H]) InitMetrics() error {
	responseSize, err := meter.
		SyncFloat64().
		Histogram(
			"header_p2p_headers_response_size",
			instrument.WithDescription("Size of get headers response in bytes"),
		)
	if err != nil {
		return err
	}

	responseDuration, err := meter.
		SyncFloat64().
		Histogram(
			"header_p2p_headers_request_duration",
			instrument.WithDescription("Duration of get headers request in seconds"),
		)
	if err != nil {
		return err
	}

	ex.metrics = &metrics{
		responseSize:     responseSize,
		responseDuration: responseDuration,
	}
	return nil
}

func (m *metrics) observeResponse(ctx context.Context, size uint64, duration uint64, err error) {
	if m == nil {
		return
	}
	m.responseSize.Record(
		ctx,
		float64(size),
		attribute.Bool("failed", err != nil),
	)
	m.responseDuration.Record(
		ctx,
		float64(duration),
		attribute.Bool("failed", err != nil),
	)
}
