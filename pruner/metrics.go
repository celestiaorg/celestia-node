package pruner

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	meter = otel.Meter("storage_pruner")
)

type metrics struct {
	prunedCounter metric.Int64Counter

	lastPruned   metric.Int64ObservableGauge
	failedPrunes metric.Int64ObservableGauge // TODO @renaynay: make callback for this
}

func (s *Service) WithMetrics() error {
	prunedCounter, err := meter.Int64Counter("pruner_pruned_counter",
		metric.WithDescription("pruner pruned header counter"))
	if err != nil {
		return err
	}

	failedPrunes, err := meter.Int64ObservableGauge("pruner_failed_counter",
		metric.WithDescription("pruner failed prunes counter"))
	if err != nil {
		return err
	}

	lastPruned, err := meter.Int64ObservableGauge("pruner_last_pruned",
		metric.WithDescription("pruner highest pruned height"))
	if err != nil {
		return err
	}

	callback := func(ctx context.Context, observer metric.Observer) error {
		// TODO @renaynay: ensure this isn't callable before pruner Start
		observer.ObserveInt64(failedPrunes, int64(len(s.checkpoint.FailedHeaders)))
		return nil
	}

	if _, err := meter.RegisterCallback(callback, failedPrunes); err != nil {
		return err
	}

	callback = func(ctx context.Context, observer metric.Observer) error {
		lastPrunedHeader := s.checkpoint.lastPrunedHeader.Load()
		if lastPrunedHeader != nil {
			observer.ObserveInt64(lastPruned, int64(lastPrunedHeader.Height()))
		}
		return nil
	}

	if _, err := meter.RegisterCallback(callback, lastPruned); err != nil {
		return err
	}

	s.metrics = &metrics{
		prunedCounter: prunedCounter,
		lastPruned:    lastPruned,
		failedPrunes:  failedPrunes,
	}
	return nil
}

func (m *metrics) observePrune(ctx context.Context, failed bool) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}
	m.prunedCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool("failed", failed)))
}
