package txclient

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("state")

const (
	attrErrorType = "error_type"
)

const (
	errorTypeTimeout  = "timeout"
	errorTypeCanceled = "canceled"
	errorTypeUnknown  = "unknown"
)

type metrics struct {
	pfbSubmissionDuration    metric.Float64Histogram
	pfbSubmissionBlobCount   metric.Int64Counter
	pfbSubmissionBlobSize    metric.Int64Counter
	pfbGasEstimationDuration metric.Float64Histogram
	pfbGasPriceEstimation    metric.Float64Histogram
	pfbSubmissionTotal       metric.Int64Counter

	gasEstimationDuration metric.Float64Histogram
	gasEstimationTotal    metric.Int64Counter

	gasPriceEstimationDuration metric.Float64Histogram
	gasPriceEstimationTotal    metric.Int64Counter

	accountQueryDuration metric.Float64Histogram
	accountQueryTotal    metric.Int64Counter
}

func (c *TxClient) WithMetrics() error {
	pfbSubmissionDuration, err := meter.Float64Histogram(
		"state_pfb_submission_duration_seconds",
		metric.WithDescription("Duration of PayForBlob submission operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	pfbSubmissionBlobCount, err := meter.Int64Counter(
		"state_pfb_submission_blob_count_total",
		metric.WithDescription("Total number of blobs in PayForBlob submissions"),
	)
	if err != nil {
		return err
	}

	pfbSubmissionBlobSize, err := meter.Int64Counter(
		"state_pfb_submission_blob_size_bytes_total",
		metric.WithDescription("Total size of blobs in PayForBlob submissions in bytes"),
	)
	if err != nil {
		return err
	}

	pfbGasEstimationDuration, err := meter.Float64Histogram(
		"state_pfb_gas_estimation_duration_seconds",
		metric.WithDescription("Duration of gas estimation for PayForBlob"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	pfbGasPriceEstimation, err := meter.Float64Histogram(
		"state_pfb_gas_price_estimation",
		metric.WithDescription("Estimated gas price for PayForBlob"),
		metric.WithUnit("utia"),
	)
	if err != nil {
		return err
	}

	gasEstimationDuration, err := meter.Float64Histogram(
		"state_gas_estimation_duration_seconds",
		metric.WithDescription("Duration of gas estimation operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	gasPriceEstimationDuration, err := meter.Float64Histogram(
		"state_gas_price_estimation_duration_seconds",
		metric.WithDescription("Duration of gas price estimation operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	accountQueryDuration, err := meter.Float64Histogram(
		"state_account_query_duration_seconds",
		metric.WithDescription("Duration of account query operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	pfbSubmissionTotal, err := meter.Int64Counter(
		"state_pfb_submission_total",
		metric.WithDescription("Total number of PayForBlob submissions"),
	)
	if err != nil {
		return err
	}

	gasEstimationTotal, err := meter.Int64Counter(
		"state_gas_estimation_total",
		metric.WithDescription("Total number of gas estimation operations"),
	)
	if err != nil {
		return err
	}

	gasPriceEstimationTotal, err := meter.Int64Counter(
		"state_gas_price_estimation_total",
		metric.WithDescription("Total number of gas price estimation operations"),
	)
	if err != nil {
		return err
	}

	accountQueryTotal, err := meter.Int64Counter(
		"state_account_query_total",
		metric.WithDescription("Total number of account query operations"),
	)
	if err != nil {
		return err
	}

	c.metrics = &metrics{
		pfbSubmissionDuration:      pfbSubmissionDuration,
		pfbSubmissionBlobCount:     pfbSubmissionBlobCount,
		pfbSubmissionBlobSize:      pfbSubmissionBlobSize,
		pfbGasEstimationDuration:   pfbGasEstimationDuration,
		pfbGasPriceEstimation:      pfbGasPriceEstimation,
		gasEstimationDuration:      gasEstimationDuration,
		gasPriceEstimationDuration: gasPriceEstimationDuration,
		accountQueryDuration:       accountQueryDuration,
		pfbSubmissionTotal:         pfbSubmissionTotal,
		gasEstimationTotal:         gasEstimationTotal,
		gasPriceEstimationTotal:    gasPriceEstimationTotal,
		accountQueryTotal:          accountQueryTotal,
	}
	return nil
}

func (m *metrics) observePfbSubmission(
	ctx context.Context,
	duration time.Duration,
	blobCount int,
	totalSize int64,
	gasEstimationDuration time.Duration,
	gasPrice float64,
	err error,
) {
	if m == nil {
		return
	}

	attrs := errorAttrs(err)
	m.pfbSubmissionDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.pfbSubmissionBlobCount.Add(ctx, int64(blobCount), metric.WithAttributes(attrs...))
	m.pfbSubmissionBlobSize.Add(ctx, totalSize, metric.WithAttributes(attrs...))
	m.pfbGasEstimationDuration.Record(ctx, gasEstimationDuration.Seconds(), metric.WithAttributes(attrs...))
	m.pfbGasPriceEstimation.Record(ctx, gasPrice, metric.WithAttributes(attrs...))
	m.pfbSubmissionTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

func (m *metrics) observeGasEstimation(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

	attrs := errorAttrs(err)
	m.gasEstimationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.gasEstimationTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

func (m *metrics) observeGasPriceEstimation(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

	attrs := errorAttrs(err)
	m.gasPriceEstimationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.gasPriceEstimationTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

func (m *metrics) observeAccountQuery(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

	attrs := errorAttrs(err)
	m.accountQueryDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.accountQueryTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

func errorAttrs(err error) []attribute.KeyValue {
	if err == nil {
		return nil
	}
	errorType := errorTypeUnknown
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		errorType = errorTypeTimeout
	case errors.Is(err, context.Canceled):
		errorType = errorTypeCanceled
	}
	return []attribute.KeyValue{attribute.String(attrErrorType, errorType)}
}
