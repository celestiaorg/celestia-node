package fibre

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("fibre")

const (
	attrErrorType = "error_type"

	errorTypeUnavailable = "unavailable"
	errorTypeTimeout     = "timeout"
	errorTypeCanceled    = "canceled"
	errorTypeUnknown     = "unknown"
)

type blobMetrics struct {
	uploadDuration metric.Float64Histogram
	submitDuration metric.Float64Histogram
}

func (s *Service) WithMetrics() error {
	err := s.withBlobMetrics()
	if err != nil {
		return err
	}
	return s.AccountClient.WithMetrics()
}

func (s *Service) withBlobMetrics() error {
	uploadDuration, err := meter.Float64Histogram(
		"fibre_upload_duration_seconds",
		metric.WithDescription("Duration of fibre blob upload operations"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	submitDuration, err := meter.Float64Histogram(
		"fibre_submit_duration_seconds",
		metric.WithDescription("Duration of fibre blob submit operations (upload + on-chain MsgPayForFibre)"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	s.metrics = &blobMetrics{
		uploadDuration: uploadDuration,
		submitDuration: submitDuration,
	}
	return nil
}

func (m *blobMetrics) observeUpload(ctx context.Context, dur time.Duration, blobSize int, err error) {
	if m == nil {
		return
	}
	m.uploadDuration.Record(ctx, dur.Seconds(), blobAttrs(blobSize, err))
}

func (m *blobMetrics) observeSubmit(ctx context.Context, dur time.Duration, blobSize int, err error) {
	if m == nil {
		return
	}
	m.submitDuration.Record(ctx, dur.Seconds(), blobAttrs(blobSize, err))
}

func blobAttrs(blobSize int, err error) metric.MeasurementOption {
	attrs := []attribute.KeyValue{
		attribute.Int("blob_size", blobSize),
	}
	if err != nil {
		attrs = append(attrs, attribute.String(attrErrorType, classifyError(err)))
	}
	return metric.WithAttributes(attrs...)
}

func errorAttrs(err error) metric.MeasurementOption {
	if err == nil {
		return metric.WithAttributes()
	}
	return metric.WithAttributes(
		attribute.String(attrErrorType, classifyError(err)),
	)
}

func classifyError(err error) string {
	switch {
	case errors.Is(err, ErrClientNotAvailable):
		return errorTypeUnavailable
	case errors.Is(err, context.DeadlineExceeded):
		return errorTypeTimeout
	case errors.Is(err, context.Canceled):
		return errorTypeCanceled
	default:
		return errorTypeUnknown
	}
}
