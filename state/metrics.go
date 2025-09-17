package state

import (
	"context"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/fx"
)

var meter = otel.Meter("state")

// Metrics tracks state-related metrics
type Metrics struct {
	// PFB submission metrics
	pfbSubmissionCounter     metric.Int64Counter
	pfbSubmissionDuration    metric.Float64Histogram
	pfbSubmissionErrors      metric.Int64Counter
	pfbSubmissionBlobCount   metric.Int64Counter
	pfbSubmissionBlobSize    metric.Int64Counter
	pfbGasEstimationDuration metric.Float64Histogram
	pfbGasPriceEstimation    metric.Float64Histogram

	// Gas estimation metrics
	gasEstimationCounter  metric.Int64Counter
	gasEstimationDuration metric.Float64Histogram
	gasEstimationErrors   metric.Int64Counter

	// Gas price estimation metrics
	gasPriceEstimationCounter  metric.Int64Counter
	gasPriceEstimationDuration metric.Float64Histogram
	gasPriceEstimationErrors   metric.Int64Counter

	// Account operations metrics
	accountQueryCounter  metric.Int64Counter
	accountQueryDuration metric.Float64Histogram
	accountQueryErrors   metric.Int64Counter

	// Internal counters (thread-safe)
	totalPfbSubmissions           atomic.Int64
	totalPfbSubmissionErrors      atomic.Int64
	totalGasEstimations           atomic.Int64
	totalGasEstimationErrors      atomic.Int64
	totalGasPriceEstimations      atomic.Int64
	totalGasPriceEstimationErrors atomic.Int64
	totalAccountQueries           atomic.Int64
	totalAccountQueryErrors       atomic.Int64
}

func WithMetrics(lc fx.Lifecycle, ca *CoreAccessor) (*Metrics, error) {
	// PFB submission metrics
	pfbSubmissionCounter, err := meter.Int64Counter(
		"state_pfb_submission_total",
		metric.WithDescription("Total number of PayForBlob submissions"),
	)
	if err != nil {
		return nil, err
	}

	pfbSubmissionDuration, err := meter.Float64Histogram(
		"state_pfb_submission_duration_seconds",
		metric.WithDescription("Duration of PayForBlob submission operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	pfbSubmissionErrors, err := meter.Int64Counter(
		"state_pfb_submission_errors_total",
		metric.WithDescription("Total number of PayForBlob submission errors"),
	)
	if err != nil {
		return nil, err
	}

	pfbSubmissionBlobCount, err := meter.Int64Counter(
		"state_pfb_submission_blob_count_total",
		metric.WithDescription("Total number of blobs in PayForBlob submissions"),
	)
	if err != nil {
		return nil, err
	}

	pfbSubmissionBlobSize, err := meter.Int64Counter(
		"state_pfb_submission_blob_size_bytes_total",
		metric.WithDescription("Total size of blobs in PayForBlob submissions in bytes"),
	)
	if err != nil {
		return nil, err
	}

	pfbGasEstimationDuration, err := meter.Float64Histogram(
		"state_pfb_gas_estimation_duration_seconds",
		metric.WithDescription("Duration of gas estimation for PayForBlob"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	pfbGasPriceEstimation, err := meter.Float64Histogram(
		"state_pfb_gas_price_estimation",
		metric.WithDescription("Estimated gas price for PayForBlob"),
		metric.WithUnit("utia"),
	)
	if err != nil {
		return nil, err
	}

	// Gas estimation metrics
	gasEstimationCounter, err := meter.Int64Counter(
		"state_gas_estimation_total",
		metric.WithDescription("Total number of gas estimation operations"),
	)
	if err != nil {
		return nil, err
	}

	gasEstimationDuration, err := meter.Float64Histogram(
		"state_gas_estimation_duration_seconds",
		metric.WithDescription("Duration of gas estimation operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	gasEstimationErrors, err := meter.Int64Counter(
		"state_gas_estimation_errors_total",
		metric.WithDescription("Total number of gas estimation errors"),
	)
	if err != nil {
		return nil, err
	}

	// Gas price estimation metrics
	gasPriceEstimationCounter, err := meter.Int64Counter(
		"state_gas_price_estimation_total",
		metric.WithDescription("Total number of gas price estimation operations"),
	)
	if err != nil {
		return nil, err
	}

	gasPriceEstimationDuration, err := meter.Float64Histogram(
		"state_gas_price_estimation_duration_seconds",
		metric.WithDescription("Duration of gas price estimation operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	gasPriceEstimationErrors, err := meter.Int64Counter(
		"state_gas_price_estimation_errors_total",
		metric.WithDescription("Total number of gas price estimation errors"),
	)
	if err != nil {
		return nil, err
	}

	// Account operations metrics
	accountQueryCounter, err := meter.Int64Counter(
		"state_account_query_total",
		metric.WithDescription("Total number of account query operations"),
	)
	if err != nil {
		return nil, err
	}

	accountQueryDuration, err := meter.Float64Histogram(
		"state_account_query_duration_seconds",
		metric.WithDescription("Duration of account query operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	accountQueryErrors, err := meter.Int64Counter(
		"state_account_query_errors_total",
		metric.WithDescription("Total number of account query errors"),
	)
	if err != nil {
		return nil, err
	}

	metrics := &Metrics{
		pfbSubmissionCounter:       pfbSubmissionCounter,
		pfbSubmissionDuration:      pfbSubmissionDuration,
		pfbSubmissionErrors:        pfbSubmissionErrors,
		pfbSubmissionBlobCount:     pfbSubmissionBlobCount,
		pfbSubmissionBlobSize:      pfbSubmissionBlobSize,
		pfbGasEstimationDuration:   pfbGasEstimationDuration,
		pfbGasPriceEstimation:      pfbGasPriceEstimation,
		gasEstimationCounter:       gasEstimationCounter,
		gasEstimationDuration:      gasEstimationDuration,
		gasEstimationErrors:        gasEstimationErrors,
		gasPriceEstimationCounter:  gasPriceEstimationCounter,
		gasPriceEstimationDuration: gasPriceEstimationDuration,
		gasPriceEstimationErrors:   gasPriceEstimationErrors,
		accountQueryCounter:        accountQueryCounter,
		accountQueryDuration:       accountQueryDuration,
		accountQueryErrors:         accountQueryErrors,
	}

	// Register observable metrics for OTLP export
	pfbSubmissionObservable, err := meter.Int64ObservableCounter(
		"state_pfb_submission_total_observable",
		metric.WithDescription("Observable total number of PayForBlob submissions"),
	)
	if err != nil {
		return nil, err
	}

	gasEstimationObservable, err := meter.Int64ObservableCounter(
		"state_gas_estimation_total_observable",
		metric.WithDescription("Observable total number of gas estimation operations"),
	)
	if err != nil {
		return nil, err
	}

	gasPriceEstimationObservable, err := meter.Int64ObservableCounter(
		"state_gas_price_estimation_total_observable",
		metric.WithDescription("Observable total number of gas price estimation operations"),
	)
	if err != nil {
		return nil, err
	}

	accountQueryObservable, err := meter.Int64ObservableCounter(
		"state_account_query_total_observable",
		metric.WithDescription("Observable total number of account query operations"),
	)
	if err != nil {
		return nil, err
	}

	// Register observable metrics for backward compatibility
	pfbCounter, _ := meter.Int64ObservableCounter(
		"pfb_count",
		metric.WithDescription("Total count of submitted PayForBlob transactions"),
	)
	lastPfbTimestamp, _ := meter.Int64ObservableCounter(
		"last_pfb_timestamp",
		metric.WithDescription("Timestamp of the last submitted PayForBlob transaction"),
	)

	callback := func(_ context.Context, observer metric.Observer) error {
		// New observable metrics
		observer.ObserveInt64(pfbSubmissionObservable, metrics.totalPfbSubmissions.Load())
		observer.ObserveInt64(gasEstimationObservable, metrics.totalGasEstimations.Load())
		observer.ObserveInt64(gasPriceEstimationObservable, metrics.totalGasPriceEstimations.Load())
		observer.ObserveInt64(accountQueryObservable, metrics.totalAccountQueries.Load())

		// Legacy observable metrics
		observer.ObserveInt64(pfbCounter, ca.PayForBlobCount())
		observer.ObserveInt64(lastPfbTimestamp, ca.LastPayForBlob())
		return nil
	}

	clientReg, err := meter.RegisterCallback(callback, pfbSubmissionObservable, gasEstimationObservable, gasPriceEstimationObservable, accountQueryObservable, pfbCounter, lastPfbTimestamp)
	if err != nil {
		log.Errorf("Failed to register metrics callback: %v", err)
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			if err := clientReg.Unregister(); err != nil {
				log.Warnw("failed to close metrics", "err", err)
			}
			return nil
		},
	})

	// Update the CoreAccessor with the new metrics
	ca.SetMetrics(metrics)

	return metrics, nil
}

// ObservePfbSubmission records PayForBlob submission metrics
func (m *Metrics) ObservePfbSubmission(
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

	// Update counters
	m.totalPfbSubmissions.Add(1)
	if err != nil {
		m.totalPfbSubmissionErrors.Add(1)
	}

	// Record metrics
	attrs := []attribute.KeyValue{
		attribute.Int("blob_count", blobCount),
		attribute.Int64("total_size_bytes", totalSize),
		attribute.Float64("gas_price_utia", gasPrice),
	}

	if err != nil {
		attrs = append(attrs, attribute.String("error", err.Error()))
		m.pfbSubmissionErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
	} else {
		m.pfbSubmissionCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	m.pfbSubmissionDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.pfbSubmissionBlobCount.Add(ctx, int64(blobCount), metric.WithAttributes(attrs...))
	m.pfbSubmissionBlobSize.Add(ctx, totalSize, metric.WithAttributes(attrs...))
	m.pfbGasEstimationDuration.Record(ctx, gasEstimationDuration.Seconds(), metric.WithAttributes(attrs...))
	m.pfbGasPriceEstimation.Record(ctx, gasPrice, metric.WithAttributes(attrs...))
}

// ObserveGasEstimation records gas estimation metrics
func (m *Metrics) ObserveGasEstimation(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

	// Update counters
	m.totalGasEstimations.Add(1)
	if err != nil {
		m.totalGasEstimationErrors.Add(1)
	}

	// Record metrics
	attrs := []attribute.KeyValue{}
	if err != nil {
		attrs = append(attrs, attribute.String("error", err.Error()))
		m.gasEstimationErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
	} else {
		m.gasEstimationCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	m.gasEstimationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// ObserveGasPriceEstimation records gas price estimation metrics
func (m *Metrics) ObserveGasPriceEstimation(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

	// Update counters
	m.totalGasPriceEstimations.Add(1)
	if err != nil {
		m.totalGasPriceEstimationErrors.Add(1)
	}

	// Record metrics
	attrs := []attribute.KeyValue{}
	if err != nil {
		attrs = append(attrs, attribute.String("error", err.Error()))
		m.gasPriceEstimationErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
	} else {
		m.gasPriceEstimationCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	m.gasPriceEstimationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// ObserveAccountQuery records account query metrics
func (m *Metrics) ObserveAccountQuery(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

	// Update counters
	m.totalAccountQueries.Add(1)
	if err != nil {
		m.totalAccountQueryErrors.Add(1)
	}

	// Record metrics
	attrs := []attribute.KeyValue{}
	if err != nil {
		attrs = append(attrs, attribute.String("error", err.Error()))
		m.accountQueryErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
	} else {
		m.accountQueryCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	m.accountQueryDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}
