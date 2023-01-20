package state

import (
	"context"

	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
)

var meter = global.MeterProvider().Meter("state")

func WithMetrics(ca *CoreAccessor) {
	pfbCounter, _ := meter.AsyncInt64().Counter(
		"pfb_count",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of submitted PayForBlob transactions"),
	)
	lastPfbTimestamp, _ := meter.AsyncInt64().Counter(
		"last_pfb_timestamp",
		instrument.WithUnit(unit.Milliseconds),
		instrument.WithDescription("Timestamp of the last submitted PayForBlob transaction"),
	)

	err := meter.RegisterCallback(
		[]instrument.Asynchronous{pfbCounter, lastPfbTimestamp},
		func(ctx context.Context) {
			pfbCounter.Observe(ctx, ca.payForBlobCount)
			lastPfbTimestamp.Observe(ctx, ca.lastPayForBlob)
		},
	)
	if err != nil {
		panic(err)
	}
}
