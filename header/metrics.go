package header

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"

	libhead "github.com/celestiaorg/celestia-node/libs/header"
)

var meter = global.MeterProvider().Meter("header")

// WithMetrics enables Otel metrics to monitor head.
func WithMetrics(store libhead.Store[*ExtendedHeader]) {
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
			head, err := store.Head(ctx)
			if err != nil {
				headC.Observe(ctx, 0, attribute.String("err", err.Error()))
				return
			}

			headC.Observe(
				ctx,
				head.Height(),
				attribute.Int("square_size", len(head.DAH.RowsRoots)),
			)
		},
	)
	if err != nil {
		panic(err)
	}
}
