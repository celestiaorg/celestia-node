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
	failedPrunes metric.Int64ObservableGauge

	clientReg metric.Registration
}

func (s *Service) WithMetrics() error {
	prunedCounter, err := meter.Int64Counter("prnr_pruned_counter",
		metric.WithDescription("pruner pruned header counter"))
	if err != nil {
		return err
	}

	failedPrunes, err := meter.Int64ObservableGauge("prnr_failed_counter",
		metric.WithDescription("pruner failed prunes counter"))
	if err != nil {
		return err
	}

	lastPruned, err := meter.Int64ObservableGauge("prnr_last_pruned",
		metric.WithDescription("pruner highest pruned height"))
	if err != nil {
		return err
	}

	callback := func(_ context.Context, observer metric.Observer) error {
		observer.ObserveInt64(lastPruned, int64(s.checkpoint.LastPrunedHeight))
		observer.ObserveInt64(failedPrunes, int64(len(s.checkpoint.FailedHeaders)))
		return nil
	}

	clientReg, err := meter.RegisterCallback(callback, lastPruned, failedPrunes)
	if err != nil {
		return err
	}

	s.metrics = &metrics{
		prunedCounter: prunedCounter,
		lastPruned:    lastPruned,
		failedPrunes:  failedPrunes,
		clientReg:     clientReg,
	}
	return nil
}

func (m *metrics) close() error {
	if m == nil {
		return nil
	}

	return m.clientReg.Unregister()
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
