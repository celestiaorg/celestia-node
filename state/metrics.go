package state

import (
	"context"

	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
)

var meter = global.MeterProvider().Meter("state")

func MonitorPFDs(ca *CoreAccessor) {
	pfdCounter, _ := meter.AsyncInt64().Counter(
		"pfd_count",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of submitted PayForData transactions"),
	)
	lastPfdTimestamp, _ := meter.AsyncInt64().Counter(
		"last_pfd_timestamp",
		instrument.WithUnit(unit.Milliseconds),
		instrument.WithDescription("Timestamp of the last submitted PayForData transaction"),
	)

	err := meter.RegisterCallback(
		[]instrument.Asynchronous{pfdCounter, lastPfdTimestamp},
		func(ctx context.Context) {
			pfdCounter.Observe(ctx, ca.payForDataCount)
			lastPfdTimestamp.Observe(ctx, ca.lastPayForData)
		},
	)
	if err != nil {
		panic(err)
	}
}
