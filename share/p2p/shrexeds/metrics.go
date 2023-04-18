package shrexeds

import (
	"context"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.opentelemetry.io/otel/metric/unit"
)

var meter = global.MeterProvider().Meter("shrex/eds")

type status string

const (
	statusNotFound    status = "NotFound"
	statusSuccess     status = "Success"
	statusRateLimited status = "RateLimited"
)

type metrics struct {
	totalRequestCounter syncint64.Counter
}

func (m *metrics) observeRequest(ctx context.Context, status status) {
	if m == nil {
		return
	}

	m.totalRequestCounter.Add(ctx, 1, attribute.String("status", string(status)))
}

func (m *metrics) observeRequestForPeer(ctx context.Context, status status, peer peer.ID) {
	if m == nil {
		return
	}

	m.totalRequestCounter.Add(ctx, 1,
		attribute.String("status", string(status)),
		attribute.String("peer", peer.String()),
	)
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

	return &metrics{
		totalRequestCounter: totalRequestCounter,
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

	return &metrics{
		totalRequestCounter: totalRequestCounter,
	}, nil
}
