package fibre

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type accountMetrics struct {
	queryDuration              metric.Float64Histogram
	depositDuration            metric.Float64Histogram
	withdrawDuration           metric.Float64Histogram
	pendingWithdrawalsDuration metric.Float64Histogram
}

func (a *AccountClient) withMetrics() error {
	queryDuration, err := meter.Float64Histogram(
		"fibre_escrow_query_duration_seconds",
		metric.WithDescription("Duration of fibre escrow account query operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	depositDuration, err := meter.Float64Histogram(
		"fibre_escrow_deposit_duration_seconds",
		metric.WithDescription("Duration of fibre escrow deposit operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	withdrawDuration, err := meter.Float64Histogram(
		"fibre_escrow_withdraw_duration_seconds",
		metric.WithDescription("Duration of fibre escrow withdrawal request operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	pendingWithdrawalsDuration, err := meter.Float64Histogram(
		"fibre_escrow_pending_withdrawals_duration_seconds",
		metric.WithDescription("Duration of fibre pending withdrawals query operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	a.metrics = &accountMetrics{
		queryDuration:              queryDuration,
		depositDuration:            depositDuration,
		withdrawDuration:           withdrawDuration,
		pendingWithdrawalsDuration: pendingWithdrawalsDuration,
	}
	return nil
}

func (m *accountMetrics) observeQuery(ctx context.Context, dur time.Duration, err error) {
	if m == nil {
		return
	}
	m.queryDuration.Record(ctx, dur.Seconds(), errorAttrs(err))
}

func (m *accountMetrics) observeDeposit(ctx context.Context, dur time.Duration, denom string, err error) {
	if m == nil {
		return
	}
	m.depositDuration.Record(ctx, dur.Seconds(), denomAttrs(denom, err))
}

func (m *accountMetrics) observeWithdraw(ctx context.Context, dur time.Duration, denom string, err error) {
	if m == nil {
		return
	}
	m.withdrawDuration.Record(ctx, dur.Seconds(), denomAttrs(denom, err))
}

func (m *accountMetrics) observePendingWithdrawals(ctx context.Context, dur time.Duration, err error) {
	if m == nil {
		return
	}
	m.pendingWithdrawalsDuration.Record(ctx, dur.Seconds(), errorAttrs(err))
}

func denomAttrs(denom string, err error) metric.MeasurementOption {
	attrs := []attribute.KeyValue{
		attribute.String("denom", denom),
	}
	if err != nil {
		attrs = append(attrs, attribute.String(attrErrorType, classifyError(err)))
	}
	return metric.WithAttributes(attrs...)
}
