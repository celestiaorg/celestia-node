package state

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/fx"
)

var meter = otel.Meter("state")

func WithMetrics(lc fx.Lifecycle, ca *CoreAccessor) {
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

	clientReg, err := meter.RegisterCallback(callback, pfbCounter, lastPfbTimestamp)
	if err != nil {
		panic(err)
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			if err := clientReg.Unregister(); err != nil {
				log.Warnw("failed to close metrics", "err", err)
			}
			return nil
		},
	})
}
