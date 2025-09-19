package blob

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("blob")

// metrics tracks blob-related metrics
type metrics struct {
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
	totalRetrievals      atomic.Int64
	totalRetrievalErrors atomic.Int64
	totalProofs          atomic.Int64
	totalProofErrors     atomic.Int64

	// Client registration for cleanup
	clientReg metric.Registration
}

// WithMetrics initializes metrics for the Service
func (s *Service) WithMetrics() error {
	// Retrieval metrics
	retrievalCounter, err := meter.Int64Counter(
		"blob_retrieval_total",
		metric.WithDescription("Total number of blob retrieval operations"),
	)
	if err != nil {
		return err
	}

	retrievalDuration, err := meter.Float64Histogram(
		"blob_retrieval_duration_seconds",
		metric.WithDescription("Duration of blob retrieval operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	retrievalErrors, err := meter.Int64Counter(
		"blob_retrieval_errors_total",
		metric.WithDescription("Total number of blob retrieval errors"),
	)
	if err != nil {
		return err
	}

	retrievalNotFound, err := meter.Int64Counter(
		"blob_retrieval_not_found_total",
		metric.WithDescription("Total number of blob not found errors"),
	)
	if err != nil {
		return err
	}

	// Proof metrics
	proofCounter, err := meter.Int64Counter(
		"blob_proof_total",
		metric.WithDescription("Total number of blob proof operations"),
	)
	if err != nil {
		return err
	}

	proofDuration, err := meter.Float64Histogram(
		"blob_proof_duration_seconds",
		metric.WithDescription("Duration of blob proof operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	proofErrors, err := meter.Int64Counter(
		"blob_proof_errors_total",
		metric.WithDescription("Total number of blob proof errors"),
	)
	if err != nil {
		return err
	}

	m := &metrics{
		retrievalCounter:  retrievalCounter,
		retrievalDuration: retrievalDuration,
		retrievalErrors:   retrievalErrors,
		retrievalNotFound: retrievalNotFound,
		proofCounter:      proofCounter,
		proofDuration:     proofDuration,
		proofErrors:       proofErrors,
	}

	// Register observable metrics
	retrievalTotal, err := meter.Int64ObservableCounter(
		"blob_retrieval_total_observable",
		metric.WithDescription("Observable total number of blob retrievals"),
	)
	if err != nil {
		return err
	}

	proofTotal, err := meter.Int64ObservableCounter(
		"blob_proof_total_observable",
		metric.WithDescription("Observable total number of blob proofs"),
	)
	if err != nil {
		return err
	}

	callback := func(_ context.Context, observer metric.Observer) error {
		observer.ObserveInt64(retrievalTotal, m.totalRetrievals.Load())
		observer.ObserveInt64(proofTotal, m.totalProofs.Load())
		return nil
	}

	clientReg, err := meter.RegisterCallback(callback, retrievalTotal, proofTotal)
	if err != nil {
		return err
	}

	m.clientReg = clientReg
	s.metrics = m
	return nil
}

// Stop cleans up metrics resources
func (m *metrics) Stop() error {
	if m == nil || m.clientReg == nil {
		return nil
	}
	return m.clientReg.Unregister()
}

// ObserveRetrieval records blob retrieval metrics
func (m *metrics) ObserveRetrieval(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

	// Update counters
	m.totalRetrievals.Add(1)
	if err != nil {
		m.totalRetrievalErrors.Add(1)
	}

	// Record metrics with error type enum to avoid cardinality explosion
	attrs := []attribute.KeyValue{}
	if err != nil {
		errorType := "unknown"
		if errors.Is(err, ErrBlobNotFound) {
			errorType = "not_found"
		} else if errors.Is(err, context.DeadlineExceeded) {
			errorType = "timeout"
		} else if errors.Is(err, context.Canceled) {
			errorType = "cancelled"
		}
		attrs = append(attrs, attribute.String("error_type", errorType))

		if errors.Is(err, ErrBlobNotFound) {
			m.retrievalNotFound.Add(ctx, 1, metric.WithAttributes(attrs...))
		} else {
			m.retrievalErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
		}
	} else {
		attrs = append(attrs, attribute.String("error_type", "none"))
		m.retrievalCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	m.retrievalDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// ObserveProof records blob proof metrics
func (m *metrics) ObserveProof(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

	// Update counters
	m.totalProofs.Add(1)
	if err != nil {
		m.totalProofErrors.Add(1)
	}

	// Record metrics with error type enum to avoid cardinality explosion
	attrs := []attribute.KeyValue{}
	if err != nil {
		errorType := "unknown"
		if errors.Is(err, context.DeadlineExceeded) {
			errorType = "timeout"
		} else if errors.Is(err, context.Canceled) {
			errorType = "cancelled"
		}
		attrs = append(attrs, attribute.String("error_type", errorType))
		m.proofErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
	} else {
		attrs = append(attrs, attribute.String("error_type", "none"))
		m.proofCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	m.proofDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}
