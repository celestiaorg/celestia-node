package state

import (
	"context"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("state")

// metrics tracks state-related metrics
type metrics struct {
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

	// Client registration for cleanup
	clientReg metric.Registration
}

// WithMetrics initializes metrics for the CoreAccessor
func (ca *CoreAccessor) WithMetrics() error {
	// PFB submission metrics
	pfbSubmissionCounter, err := meter.Int64Counter(
		"state_pfb_submission_total",
		metric.WithDescription("Total number of PayForBlob submissions"),
	)
	if err != nil {
		return err
	}

	pfbSubmissionDuration, err := meter.Float64Histogram(
		"state_pfb_submission_duration_seconds",
		metric.WithDescription("Duration of PayForBlob submission operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	pfbSubmissionErrors, err := meter.Int64Counter(
		"state_pfb_submission_errors_total",
		metric.WithDescription("Total number of PayForBlob submission errors"),
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

	// Gas estimation metrics
	gasEstimationCounter, err := meter.Int64Counter(
		"state_gas_estimation_total",
		metric.WithDescription("Total number of gas estimation operations"),
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

	gasEstimationErrors, err := meter.Int64Counter(
		"state_gas_estimation_errors_total",
		metric.WithDescription("Total number of gas estimation errors"),
	)
	if err != nil {
		return err
	}

	// Gas price estimation metrics
	gasPriceEstimationCounter, err := meter.Int64Counter(
		"state_gas_price_estimation_total",
		metric.WithDescription("Total number of gas price estimation operations"),
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

	gasPriceEstimationErrors, err := meter.Int64Counter(
		"state_gas_price_estimation_errors_total",
		metric.WithDescription("Total number of gas price estimation errors"),
	)
	if err != nil {
		return err
	}

	// Account operations metrics
	accountQueryCounter, err := meter.Int64Counter(
		"state_account_query_total",
		metric.WithDescription("Total number of account query operations"),
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

	accountQueryErrors, err := meter.Int64Counter(
		"state_account_query_errors_total",
		metric.WithDescription("Total number of account query errors"),
	)
	if err != nil {
		return err
	}

	m := &metrics{
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
		return err
	}

	gasEstimationObservable, err := meter.Int64ObservableCounter(
		"state_gas_estimation_total_observable",
		metric.WithDescription("Observable total number of gas estimation operations"),
	)
	if err != nil {
		return err
	}

	gasPriceEstimationObservable, err := meter.Int64ObservableCounter(
		"state_gas_price_estimation_total_observable",
		metric.WithDescription("Observable total number of gas price estimation operations"),
	)
	if err != nil {
		return err
	}

	accountQueryObservable, err := meter.Int64ObservableCounter(
		"state_account_query_total_observable",
		metric.WithDescription("Observable total number of account query operations"),
	)
	if err != nil {
		return err
	}

	callback := func(_ context.Context, observer metric.Observer) error {
		// Observable metrics for OTLP export
		observer.ObserveInt64(pfbSubmissionObservable, m.totalPfbSubmissions.Load())
		observer.ObserveInt64(gasEstimationObservable, m.totalGasEstimations.Load())
		observer.ObserveInt64(gasPriceEstimationObservable, m.totalGasPriceEstimations.Load())
		observer.ObserveInt64(accountQueryObservable, m.totalAccountQueries.Load())
		return nil
	}

	clientReg, err := meter.RegisterCallback(
		callback,
		pfbSubmissionObservable,
		gasEstimationObservable,
		gasPriceEstimationObservable,
		accountQueryObservable,
	)
	if err != nil {
		log.Errorf("Failed to register metrics callback: %v", err)
		return err
	}

	// Update the CoreAccessor with the new metrics
	ca.metrics = m

	// Store the client registration for cleanup
	m.clientReg = clientReg

	return nil
}

// Stop cleans up the metrics resources
func (m *metrics) Stop() error {
	if m == nil || m.clientReg == nil {
		return nil
	}
	if err := m.clientReg.Unregister(); err != nil {
		log.Warnw("failed to close metrics", "err", err)
		return err
	}
	return nil
}

// ObservePfbSubmission records PayForBlob submission metrics
func (m *metrics) ObservePfbSubmission(
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
func (m *metrics) ObserveGasEstimation(ctx context.Context, duration time.Duration, err error) {
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
func (m *metrics) ObserveGasPriceEstimation(ctx context.Context, duration time.Duration, err error) {
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
func (m *metrics) ObserveAccountQuery(ctx context.Context, duration time.Duration, err error) {
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
