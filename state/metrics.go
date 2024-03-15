package state

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("state")

func WithMetrics(ca *CoreAccessor) {
	pfbCounter, _ := meter.Int64ObservableCounter(
		"pfb_count",
		metric.WithDescription("Total count of submitted PayForBlob transactions"),
	)
	lastPfbTimestamp, _ := meter.Int64ObservableCounter(
		"last_pfb_timestamp",
		metric.WithDescription("Timestamp of the last submitted PayForBlob transaction"),
	)

	callback := func(_ context.Context, observer metric.Observer) error {
		observer.ObserveInt64(pfbCounter, ca.PayForBlobCount())
		observer.ObserveInt64(lastPfbTimestamp, ca.LastPayForBlob())
		return nil
	}
	_, err := meter.RegisterCallback(callback, pfbCounter, lastPfbTimestamp)
	if err != nil {
		panic(err)
	}
}
