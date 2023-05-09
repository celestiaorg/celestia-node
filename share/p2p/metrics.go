package p2p

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.opentelemetry.io/otel/metric/unit"
)

var meter = global.MeterProvider().Meter("shrex/eds")

type status string

const (
	StatusInternalErr status = "internal_err"
	StatusNotFound    status = "not_found"
	StatusTimeout     status = "timeout"
	StatusSuccess     status = "success"
	StatusRateLimited status = "rate_limited"
)

type Metrics struct {
	totalRequestCounter syncint64.Counter
}

// ObserveRequests increments the total number of requests sent with the given status as an
// attribute.
func (m *Metrics) ObserveRequests(ctx context.Context, count int64, status status) {
	if m == nil {
		return
	}
	if ctx.Err() != nil {
		ctx = context.Background()
	}
	m.totalRequestCounter.Add(ctx, count, attribute.String("status", string(status)))
}

func InitClientMetrics(protocol string) (*Metrics, error) {
	totalRequestCounter, err := meter.SyncInt64().Counter(
		fmt.Sprintf("shrex_%s_client_total_requests", protocol),
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription(fmt.Sprintf("Total count of sent shrex/%s requests", protocol)),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		totalRequestCounter: totalRequestCounter,
	}, nil
}

func InitServerMetrics(protocol string) (*Metrics, error) {
	totalRequestCounter, err := meter.SyncInt64().Counter(
		fmt.Sprintf("shrex_%s_server_total_responses", protocol),
		instrument.WithUnit(unit.Dimensionless),
		instrument.WithDescription(fmt.Sprintf("Total count of sent shrex/%s responses", protocol)),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		totalRequestCounter: totalRequestCounter,
	}, nil
}
