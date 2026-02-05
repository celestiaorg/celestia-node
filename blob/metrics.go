package blob

import (
	"context"
	"errors"
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
	retrievalTotal    metric.Int64Counter

	// Proof metrics
	proofDuration metric.Float64Histogram
	proofTotal    metric.Int64Counter
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

	retrievalTotal, err := meter.Int64Counter(
		"blob_retrieval_total",
		metric.WithDescription("Total number of blob retrievals"),
	)
	if err != nil {
		return err
	}

	proofTotal, err := meter.Int64Counter(
		"blob_proof_total",
		metric.WithDescription("Total number of blob proofs"),
	)
	if err != nil {
		return err
	}

	m := &metrics{
		retrievalDuration: retrievalDuration,
		retrievalTotal:    retrievalTotal,
		proofDuration:     proofDuration,
		proofTotal:        proofTotal,
	}

	s.metrics = m
	return nil
}

// observeRetrieval records blob retrieval metrics
func (m *metrics) observeRetrieval(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

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

	m.retrievalDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.retrievalTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// observeProof records blob proof metrics
func (m *metrics) observeProof(ctx context.Context, duration time.Duration, err error) {
	if m == nil {
		return
	}

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

	m.proofDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.proofTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}
