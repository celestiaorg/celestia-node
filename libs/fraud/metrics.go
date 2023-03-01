package fraud

import (
	"context"

	"github.com/ipfs/go-datastore"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
)

// WithMetrics enables metrics to monitor fraud proofs.
func WithMetrics(store Getter) {
	proofTypes := registeredProofTypes()
	for _, proofType := range proofTypes {
		counter, _ := meter.AsyncInt64().Gauge(string(proofType),
			instrument.WithUnit(unit.Dimensionless),
			instrument.WithDescription("Stored fraud proof"),
		)
		err := meter.RegisterCallback(
			[]instrument.Asynchronous{
				counter,
			},
			func(ctx context.Context) {
				proofs, err := store.Get(ctx, proofType)
				switch err {
				case nil:
					counter.Observe(ctx,
						int64(len(proofs)),
						attribute.String("proof_type", string(proofType)),
					)
				case datastore.ErrNotFound:
					counter.Observe(ctx, 0, attribute.String("err", "not_found"))
					return
				default:
					counter.Observe(ctx, 0, attribute.String("err", "unknown"))
				}
			},
		)

		if err != nil {
			panic(err)
		}
	}
}
