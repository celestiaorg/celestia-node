package blob

import (
	"context"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/fx"
)

var meter = otel.Meter("blob")

// Metrics tracks blob-related metrics
type Metrics struct {
	// Submission metrics
	submissionCounter   metric.Int64Counter
	submissionDuration  metric.Float64Histogram
	submissionErrors    metric.Int64Counter
	submissionBlobCount metric.Int64Counter
	submissionBlobSize  metric.Int64Counter

	// Retrieval metrics
	retrievalCounter  metric.Int64Counter
	retrievalDuration metric.Float64Histogram
	retrievalErrors   metric.Int64Counter
	retrievalNotFound metric.Int64Counter

	// Proof metrics
	proofCounter  metric.Int64Counter
	proofDuration metric.Float64Histogram
	proofErrors   metric.Int64Counter

	// Internal counters (thread-safe)
	totalSubmissions      atomic.Int64
	totalSubmissionErrors atomic.Int64
	totalRetrievals       atomic.Int64
	totalRetrievalErrors  atomic.Int64
	totalProofs           atomic.Int64
	totalProofErrors      atomic.Int64
}

// WithMetrics registers blob metrics
func WithMetrics(lc fx.Lifecycle) (*Metrics, error) {
	// Submission metrics
	submissionCounter, err := meter.Int64Counter(
		"blob_submission_total",
		metric.WithDescription("Total number of blob submissions"),
	)
	if err != nil {
		return nil, err
	}

	submissionDuration, err := meter.Float64Histogram(
		"blob_submission_duration_seconds",
		metric.WithDescription("Duration of blob submission operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	submissionErrors, err := meter.Int64Counter(
		"blob_submission_errors_total",
		metric.WithDescription("Total number of blob submission errors"),
	)
	if err != nil {
		return nil, err
	}

	submissionBlobCount, err := meter.Int64Counter(
		"blob_submission_blob_count_total",
		metric.WithDescription("Total number of blobs submitted"),
	)
	if err != nil {
		return nil, err
	}

	submissionBlobSize, err := meter.Int64Counter(
		"blob_submission_blob_size_bytes_total",
		metric.WithDescription("Total size of blobs submitted in bytes"),
	)
	if err != nil {
		return nil, err
	}

	// Retrieval metrics
	retrievalCounter, err := meter.Int64Counter(
		"blob_retrieval_total",
		metric.WithDescription("Total number of blob retrieval operations"),
	)
	if err != nil {
		return nil, err
	}

	retrievalDuration, err := meter.Float64Histogram(
		"blob_retrieval_duration_seconds",
		metric.WithDescription("Duration of blob retrieval operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	retrievalErrors, err := meter.Int64Counter(
		"blob_retrieval_errors_total",
		metric.WithDescription("Total number of blob retrieval errors"),
	)
	if err != nil {
		return nil, err
	}

	retrievalNotFound, err := meter.Int64Counter(
		"blob_retrieval_not_found_total",
		metric.WithDescription("Total number of blob not found errors"),
	)
	if err != nil {
		return nil, err
	}

	// Proof metrics
	proofCounter, err := meter.Int64Counter(
		"blob_proof_total",
		metric.WithDescription("Total number of blob proof operations"),
	)
	if err != nil {
		return nil, err
	}

	proofDuration, err := meter.Float64Histogram(
		"blob_proof_duration_seconds",
		metric.WithDescription("Duration of blob proof operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	proofErrors, err := meter.Int64Counter(
		"blob_proof_errors_total",
		metric.WithDescription("Total number of blob proof errors"),
	)
	if err != nil {
		return nil, err
	}

	metrics := &Metrics{
		submissionCounter:   submissionCounter,
		submissionDuration:  submissionDuration,
		submissionErrors:    submissionErrors,
		submissionBlobCount: submissionBlobCount,
		submissionBlobSize:  submissionBlobSize,
		retrievalCounter:    retrievalCounter,
		retrievalDuration:   retrievalDuration,
		retrievalErrors:     retrievalErrors,
		retrievalNotFound:   retrievalNotFound,
		proofCounter:        proofCounter,
		proofDuration:       proofDuration,
		proofErrors:         proofErrors,
	}

	// Register observable metrics
	submissionTotal, err := meter.Int64ObservableCounter(
		"blob_submission_total_observable",
		metric.WithDescription("Observable total number of blob submissions"),
	)
	if err != nil {
		return nil, err
	}

	retrievalTotal, err := meter.Int64ObservableCounter(
		"blob_retrieval_total_observable",
		metric.WithDescription("Observable total number of blob retrievals"),
	)
	if err != nil {
		return nil, err
	}

	proofTotal, err := meter.Int64ObservableCounter(
		"blob_proof_total_observable",
		metric.WithDescription("Observable total number of blob proofs"),
	)
	if err != nil {
		return nil, err
	}

	callback := func(_ context.Context, observer metric.Observer) error {
		observer.ObserveInt64(submissionTotal, metrics.totalSubmissions.Load())
		observer.ObserveInt64(retrievalTotal, metrics.totalRetrievals.Load())
		observer.ObserveInt64(proofTotal, metrics.totalProofs.Load())
		return nil
	}

	clientReg, err := meter.RegisterCallback(callback, submissionTotal, retrievalTotal, proofTotal)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return clientReg.Unregister()
		},
	})

	return metrics, nil
}

// ObserveSubmission records blob submission metrics
func (m *Metrics) ObserveSubmission(ctx context.Context, duration time.Duration, blobCount int, totalSize int64, err error) {
	if m == nil {
		return
	}

	// Update counters
	m.totalSubmissions.Add(1)
	if err != nil {
		m.totalSubmissionErrors.Add(1)
	}

	// Record metrics
	attrs := []attribute.KeyValue{
		attribute.Int("blob_count", blobCount),
		attribute.Int64("total_size_bytes", totalSize),
	}

	if err != nil {
		attrs = append(attrs, attribute.String("error", err.Error()))
		m.submissionErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
	} else {
		m.submissionCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	m.submissionDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.submissionBlobCount.Add(ctx, int64(blobCount), metric.WithAttributes(attrs...))
	m.submissionBlobSize.Add(ctx, totalSize, metric.WithAttributes(attrs...))
}

// ObserveRetrieval records blob retrieval metrics
func (m *Metrics) ObserveRetrieval(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

	// Update counters
	m.totalRetrievals.Add(1)
	if err != nil {
		m.totalRetrievalErrors.Add(1)
	}

	// Record metrics
	attrs := []attribute.KeyValue{}
	if err != nil {
		attrs = append(attrs, attribute.String("error", err.Error()))
		if err == ErrBlobNotFound {
			m.retrievalNotFound.Add(ctx, 1, metric.WithAttributes(attrs...))
		} else {
			m.retrievalErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
		}
	} else {
		m.retrievalCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	m.retrievalDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// ObserveProof records blob proof metrics
func (m *Metrics) ObserveProof(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

	// Update counters
	m.totalProofs.Add(1)
	if err != nil {
		m.totalProofErrors.Add(1)
	}

	// Record metrics
	attrs := []attribute.KeyValue{}
	if err != nil {
		attrs = append(attrs, attribute.String("error", err.Error()))
		m.proofErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
	} else {
		m.proofCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	m.proofDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}
