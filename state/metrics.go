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

// Attribute name constants for metrics labels
const (
	attrBlobCount = "blob_count"
	attrTotalSize = "total_size_bytes"
	attrGasPrice  = "gas_price_utia"
	attrSuccess   = "success"
)

// metrics tracks state-related metrics
type metrics struct {
	// PFB submission metrics
	pfbSubmissionDuration    metric.Float64Histogram
	pfbSubmissionBlobCount   metric.Int64Counter
	pfbSubmissionBlobSize    metric.Int64Counter
	pfbGasEstimationDuration metric.Float64Histogram
	pfbGasPriceEstimation    metric.Float64Histogram

	// Gas estimation metrics
	gasEstimationDuration metric.Float64Histogram

	// Gas price estimation metrics
	gasPriceEstimationDuration metric.Float64Histogram

	// Account operations metrics
	accountQueryDuration metric.Float64Histogram

	// Internal counters (thread-safe)
	totalPfbSubmissions      atomic.Int64
	totalGasEstimations      atomic.Int64
	totalGasPriceEstimations atomic.Int64
	totalAccountQueries      atomic.Int64

	// Client registration for cleanup
	clientReg metric.Registration
}

// WithMetrics initializes metrics for the CoreAccessor
func (ca *CoreAccessor) WithMetrics() error {
	// PFB submission metrics
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

	// Gas estimation metrics
	gasEstimationDuration, err := meter.Float64Histogram(
		"state_gas_estimation_duration_seconds",
		metric.WithDescription("Duration of gas estimation operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	// Gas price estimation metrics
	gasPriceEstimationDuration, err := meter.Float64Histogram(
		"state_gas_price_estimation_duration_seconds",
		metric.WithDescription("Duration of gas price estimation operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	// Account operations metrics
	accountQueryDuration, err := meter.Float64Histogram(
		"state_account_query_duration_seconds",
		metric.WithDescription("Duration of account query operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	m := &metrics{
		pfbSubmissionDuration:      pfbSubmissionDuration,
		pfbSubmissionBlobCount:     pfbSubmissionBlobCount,
		pfbSubmissionBlobSize:      pfbSubmissionBlobSize,
		pfbGasEstimationDuration:   pfbGasEstimationDuration,
		pfbGasPriceEstimation:      pfbGasPriceEstimation,
		gasEstimationDuration:      gasEstimationDuration,
		gasPriceEstimationDuration: gasPriceEstimationDuration,
		accountQueryDuration:       accountQueryDuration,
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

// stop cleans up the metrics resources
func (m *metrics) stop() error {
	if m == nil || m.clientReg == nil {
		return nil
	}
	if err := m.clientReg.Unregister(); err != nil {
		log.Warnw("failed to close metrics", "err", err)
		return err
	}
	return nil
}

// observePfbSubmission records PayForBlob submission metrics
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

	// Update counters
	m.totalPfbSubmissions.Add(1)

	attrs := []attribute.KeyValue{
		attribute.Int(attrBlobCount, blobCount),
		attribute.Int64(attrTotalSize, totalSize),
		attribute.Float64(attrGasPrice, gasPrice),
		attribute.Bool(attrSuccess, err == nil),
	}

	m.pfbSubmissionDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.pfbSubmissionBlobCount.Add(ctx, int64(blobCount), metric.WithAttributes(attrs...))
	m.pfbSubmissionBlobSize.Add(ctx, totalSize, metric.WithAttributes(attrs...))
	m.pfbGasEstimationDuration.Record(ctx, gasEstimationDuration.Seconds(), metric.WithAttributes(attrs...))
	m.pfbGasPriceEstimation.Record(ctx, gasPrice, metric.WithAttributes(attrs...))
}

// observeGasEstimation records gas estimation metrics
func (m *metrics) observeGasEstimation(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

	// Update counters
	m.totalGasEstimations.Add(1)

	attrs := []attribute.KeyValue{
		attribute.Bool(attrSuccess, err == nil),
	}

	m.gasEstimationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// observeGasPriceEstimation records gas price estimation metrics
func (m *metrics) observeGasPriceEstimation(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

	// Update counters
	m.totalGasPriceEstimations.Add(1)

	attrs := []attribute.KeyValue{
		attribute.Bool(attrSuccess, err == nil),
	}

	m.gasPriceEstimationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// observeAccountQuery records account query metrics
func (m *metrics) observeAccountQuery(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

	// Update counters
	m.totalAccountQueries.Add(1)

	attrs := []attribute.KeyValue{
		attribute.Bool(attrSuccess, err == nil),
	}

	m.accountQueryDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}
