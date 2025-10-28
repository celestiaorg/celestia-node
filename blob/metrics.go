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

// Error type constants for metrics labels
const (
	errorTypeNotFound = "not_found"
	errorTypeTimeout  = "timeout"
	errorTypeCanceled = "canceled"
	errorTypeUnknown  = "unknown"
)

// metrics tracks blob-related metrics
type metrics struct {
	// Retrieval metrics
	retrievalDuration metric.Float64Histogram

	// Proof metrics
	proofDuration metric.Float64Histogram

	// Internal counters (thread-safe)
	totalRetrievals atomic.Int64
	totalProofs     atomic.Int64

	// Client registration for cleanup
	clientReg metric.Registration
}

// WithMetrics initializes metrics for the Service
func (s *Service) WithMetrics() error {
	// Retrieval metrics
	retrievalDuration, err := meter.Float64Histogram(
		"blob_retrieval_duration_seconds",
		metric.WithDescription("Duration of blob retrieval operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	// Proof metrics
	proofDuration, err := meter.Float64Histogram(
		"blob_proof_duration_seconds",
		metric.WithDescription("Duration of blob proof operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	m := &metrics{
		retrievalDuration: retrievalDuration,
		proofDuration:     proofDuration,
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

// stop cleans up metrics resources
func (m *metrics) stop() error {
	if m == nil || m.clientReg == nil {
		return nil
	}
	return m.clientReg.Unregister()
}

// observeRetrieval records blob retrieval metrics
func (m *metrics) observeRetrieval(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

	m.totalRetrievals.Add(1)

	// Record metrics with error type enum to avoid cardinality explosion
	attrs := []attribute.KeyValue{}
	if err != nil {
		errorType := errorTypeUnknown
		switch {
		case errors.Is(err, ErrBlobNotFound):
			errorType = errorTypeNotFound
		case errors.Is(err, context.DeadlineExceeded):
			errorType = errorTypeTimeout
		case errors.Is(err, context.Canceled):
			errorType = errorTypeCanceled
		}
		attrs = append(attrs, attribute.String("error_type", errorType))
	}

	// Use single counter with error_type enum
	m.retrievalDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// observeProof records blob proof metrics
func (m *metrics) observeProof(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

	m.totalProofs.Add(1)

	attrs := []attribute.KeyValue{}
	if err != nil {
		errorType := errorTypeUnknown
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			errorType = errorTypeTimeout
		case errors.Is(err, context.Canceled):
			errorType = errorTypeCanceled
		}
		attrs = append(attrs, attribute.String("error_type", errorType))
	}

	// Use single counter with error_type enum
	m.proofDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}
