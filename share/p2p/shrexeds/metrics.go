package shrexeds

import (
	"context"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
)

var meter = global.MeterProvider().Meter("shrex/eds")

func WithClientMetrics(client *Client) {
	totalRequestCounter, _ := meter.AsyncInt64().Counter(
		"total_requests",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of sent shrex/eds requests"),
	)
	rateLimitedCounter, _ := meter.AsyncInt64().Counter(
		"ratelimited_requests",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of rate-limited responses from shrex/eds requests"),
	)
	notFoundCounter, _ := meter.AsyncInt64().Counter(
		"notfound_requests",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of `NotFound` responses from shrex/eds requests"),
	)
	successCounter, _ := meter.AsyncInt64().Counter(
		"successful_requests",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of successful shrex/eds requests"),
	)

	err := meter.RegisterCallback(
		[]instrument.Asynchronous{totalRequestCounter, rateLimitedCounter, notFoundCounter, successCounter},
		func(ctx context.Context) {
			totalRequestCounter.Observe(ctx, client.totalRequests)
			rateLimitedCounter.Observe(ctx, client.numRatelimitedRequests)
			notFoundCounter.Observe(ctx, client.numNotFoundRequests)
			successCounter.Observe(ctx, client.numSuccessfulRequests)
		},
	)
	if err != nil {
		panic(err)
	}
}

func WithServerMetrics(server *Server) {
	totalResponseCounter, _ := meter.AsyncInt64().Counter(
		"total_responses",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of sent shrex/eds responses"),
	)
	rateLimitedCounter, _ := meter.AsyncInt64().Counter(
		"ratelimited_responses",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of sent rate-limited responses for shrex/eds"),
	)
	notFoundCounter, _ := meter.AsyncInt64().Counter(
		"notfound_responses",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of sent `NotFound` responses for shrex/eds"),
	)
	successCounter, _ := meter.AsyncInt64().Counter(
		"successful_responses",
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription("Total count of successful shrex/eds responses"),
	)

	err := meter.RegisterCallback(
		[]instrument.Asynchronous{totalResponseCounter, rateLimitedCounter, notFoundCounter, successCounter},
		func(ctx context.Context) {
			totalResponseCounter.Observe(ctx, server.middleware.NumRequests)
			rateLimitedCounter.Observe(ctx, server.middleware.NumRateLimited)
			notFoundCounter.Observe(ctx, server.numNotFoundServed)
			successCounter.Observe(ctx, server.numSuccessfullyServed)
		},
	)
	if err != nil {
		panic(err)
	}
}
