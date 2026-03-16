package utils

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.11.0"
)

// MetricProviderConfig holds configuration for creating a metric provider
type MetricProviderConfig struct {
	// ServiceNamespace is the service namespace (e.g., network name, "latency-monitor")
	ServiceNamespace string
	// ServiceName is the service name (e.g., node type, "cel-shed")
	ServiceName string
	// ServiceInstanceID is the unique instance identifier (e.g., peer ID)
	ServiceInstanceID string
	// Interval is the interval at which metrics are collected and exported (defaults to 10s)
	Interval time.Duration
	// OTLPOptions are OTLP HTTP options (e.g., endpoint, headers, TLS config)
	OTLPOptions []otlpmetrichttp.Option
}

// NewMetricProvider creates a new OTLP metric provider with the given configuration
func NewMetricProvider(ctx context.Context, cfg MetricProviderConfig) (*sdk.MeterProvider, error) {
	opts := []otlpmetrichttp.Option{otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression)} //nolint:prealloc
	opts = append(opts, cfg.OTLPOptions...)

	exp, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP metric exporter: %w", err)
	}

	interval := cfg.Interval
	if interval == 0 {
		interval = 10 * time.Second
	}

	provider := sdk.NewMeterProvider(
		sdk.WithReader(
			sdk.NewPeriodicReader(exp,
				sdk.WithTimeout(interval),
				sdk.WithInterval(interval))),
		sdk.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNamespaceKey.String(cfg.ServiceNamespace),
				semconv.ServiceNameKey.String(cfg.ServiceName),
				semconv.ServiceInstanceIDKey.String(cfg.ServiceInstanceID),
			)))

	return provider, nil
}
