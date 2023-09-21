package pruner

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	meter = otel.Meter("storage_pruner")
)

type metrics struct {
	registeredCounter metric.Int64Counter
	prunedCounter     metric.Int64Counter

	epochAmount metric.Int64ObservableGauge
}

func (sp *StoragePruner) WithMetrics() error {
	registeredCounter, err := meter.Int64Counter("pruner_registered_counter",
		metric.WithDescription("pruner registered datahash counter"))
	if err != nil {
		return err
	}

	prunedCounter, err := meter.Int64Counter("pruner_pruned_counter",
		metric.WithDescription("pruner pruned datahash counter"))
	if err != nil {
		return err
	}

	epochAmount, err := meter.Int64ObservableGauge("pruner_epoch_amount",
		metric.WithDescription("pruner epoch amount"))
	if err != nil {
		return err
	}

	callback := func(ctx context.Context, observer metric.Observer) error {
		observer.ObserveInt64(epochAmount, int64(len(sp.activeEpochs)))
		return nil
	}

	if _, err := meter.RegisterCallback(callback, epochAmount); err != nil {
		return err
	}

	sp.metrics = &metrics{
		registeredCounter: registeredCounter,
		prunedCounter:     prunedCounter,
		epochAmount:       epochAmount,
	}
	return nil
}

func (m *metrics) observeRegister(ctx context.Context) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}
	m.registeredCounter.Add(context.Background(), 1)
}

func (m *metrics) observePrune(ctx context.Context) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}
	m.prunedCounter.Add(context.Background(), 1)
}
