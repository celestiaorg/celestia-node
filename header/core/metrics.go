package core

import (
	"context"

	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
)

var meter = global.MeterProvider().Meter("header/core")

func MonitorBroadcasting(listener *Listener) {
	lastHeaderBroadcasted, _ := meter.AsyncInt64().Counter(
		"last_header_broadcasted",
		instrument.WithUnit(unit.Milliseconds),
		instrument.WithDescription("Timestamp of the last header broadcasted"),
	)

	err := meter.RegisterCallback(
		[]instrument.Asynchronous{
			lastHeaderBroadcasted,
		},
		func(ctx context.Context) {
			lastHeaderBroadcasted.Observe(ctx, listener.lastHeaderBroadcast)
		},
	)
	if err != nil {
		panic(err)
	}
}
