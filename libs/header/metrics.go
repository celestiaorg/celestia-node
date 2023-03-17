package header

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
)

var meter = global.MeterProvider().Meter("header")

// WithMetrics enables Otel metrics to monitor head and total amount of synced headers.
func WithMetrics[H Header](store Store[H]) error {
	headC, _ := meter.AsyncInt64().Counter(
		"head",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Subjective head of the node"),
	)

	err := meter.RegisterCallback(
		[]instrument.Asynchronous{
			headC,
		},
		func(ctx context.Context) {
			// add timeout to limit the time it takes to get the head
			// in case there is a deadlock
			ctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			head, err := store.Head(ctx)
			if err != nil {
				headC.Observe(ctx, 0, attribute.String("err", err.Error()))
				return
			}

			headC.Observe(
				ctx,
				head.Height(),
			)
		},
	)
	return err
}
