package shrexeds

import (
	"context"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.opentelemetry.io/otel/metric/unit"
)

var meter = global.MeterProvider().Meter("shrex/eds")

type status int

const (
	statusNotFound status = iota
	statusSuccess
	statusRateLimited
)

type metrics struct {
	totalRequestCounter syncint64.Counter
	rateLimitedCounter  syncint64.Counter
	notFoundCounter     syncint64.Counter
	successCounter      syncint64.Counter
}

func (m *metrics) observeRequest(ctx context.Context, status status) {
	if m == nil {
		return
	}

	m.totalRequestCounter.Add(ctx, 1)
	switch status {
	case statusNotFound:
		m.notFoundCounter.Add(ctx, 1)
	case statusSuccess:
		m.successCounter.Add(ctx, 1)
	case statusRateLimited:
		m.rateLimitedCounter.Add(ctx, 1)
	}
}

func initClientMetrics() (*metrics, error) {
	totalRequestCounter, err := meter.SyncInt64().Counter(
		"total_shrex_requests",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of sent shrex/eds requests"),
	)
	if err != nil {
		return nil, err
	}

	rateLimitedCounter, err := meter.SyncInt64().Counter(
		"ratelimited_shrex_requests",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of rate-limited responses from shrex/eds requests"),
	)
	if err != nil {
		return nil, err
	}

	notFoundCounter, err := meter.SyncInt64().Counter(
		"notfound_shrex_requests",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of `NotFound` responses from shrex/eds requests"),
	)
	if err != nil {
		return nil, err
	}

	successCounter, err := meter.SyncInt64().Counter(
		"successful_shrex_requests",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of successful shrex/eds requests"),
	)
	if err != nil {
		return nil, err
	}

	return &metrics{
		totalRequestCounter: totalRequestCounter,
		rateLimitedCounter:  rateLimitedCounter,
		notFoundCounter:     notFoundCounter,
		successCounter:      successCounter,
	}, nil
}

func initServerMetrics() (*metrics, error) {
	totalRequestCounter, err := meter.SyncInt64().Counter(
		"total_shrex_responses",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of sent shrex/eds responses"),
	)
	if err != nil {
		return nil, err
	}

	rateLimitedCounter, err := meter.SyncInt64().Counter(
		"ratelimited_shrex_responses",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of sent rate-limited responses for shrex/eds"),
	)
	if err != nil {
		return nil, err
	}

	notFoundCounter, err := meter.SyncInt64().Counter(
		"notfound_shrex_responses",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of sent `NotFound` responses for shrex/eds"),
	)
	if err != nil {
		return nil, err
	}

	successCounter, err := meter.SyncInt64().Counter(
		"successful_shrex_responses",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of successful shrex/eds responses"),
	)
	if err != nil {
		return nil, err
	}

	return &metrics{
		totalRequestCounter: totalRequestCounter,
		rateLimitedCounter:  rateLimitedCounter,
		notFoundCounter:     notFoundCounter,
		successCounter:      successCounter,
	}, nil
}
