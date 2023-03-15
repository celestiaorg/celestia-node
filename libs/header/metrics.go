package header

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"

	"github.com/celestiaorg/celestia-node/libs/header/p2p"
	"github.com/celestiaorg/celestia-node/libs/header/sync"
)

var meter = global.MeterProvider().Meter("header")

// WithMetrics enables Otel metrics to monitor head and total amount of synced headers.
func WithMetrics[H Header](store Store[H], syncer *sync.Syncer[H], ex Exchange[H]) error {
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
	if err != nil {
		return err
	}

	if p2pex, ok := ex.(*p2p.Exchange[H]); ok {
		if err := p2pex.InitMetrics(); err != nil {
			return err
		}
	}

	return syncer.InitMetrics()
}
