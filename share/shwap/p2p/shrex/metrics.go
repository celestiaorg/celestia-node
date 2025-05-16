package shrex

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

var meter = otel.Meter("shrex")

type status string

const (
	StatusBadRequest  status = "bad_request"
	StatusSendRespErr status = "send_resp_err"
	StatusSendReqErr  status = "send_req_err"
	StatusReadRespErr status = "read_resp_err"
	StatusInternalErr status = "internal_err"
	StatusNotFound    status = "not_found"
	StatusTimeout     status = "timeout"
	StatusSuccess     status = "success"
	StatusRateLimited status = "rate_limited"
)

type Metrics struct {
	totalRequestCounter metric.Int64Counter
	requestDuration     metric.Float64Histogram
}

// observeRequests increments the total number of requests sent with the given status as an
// attribute.
func (m *Metrics) observeRequests(ctx context.Context, count int64, requestName string, status status) {
	if m == nil {
		return
	}
	ctx = utils.ResetContextOnError(ctx)
	m.totalRequestCounter.Add(ctx, count,
		metric.WithAttributes(
			attribute.String("request.name", requestName),
			attribute.String("status", string(status)),
		))
}

func (m *Metrics) observeDuration(ctx context.Context, requestName string, d time.Duration) {
	m.requestDuration.Record(ctx, d.Seconds(),
		metric.WithAttributes(
			attribute.String("request.name", requestName),
		))
}

func InitClientMetrics() (*Metrics, error) {
	totalRequestCounter, err := meter.Int64Counter(
		"shrex_client_total_requests",
		metric.WithDescription("Total count of sent shrex requests"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		"shrex_client_request_duration",
		metric.WithDescription("Time taken to complete a shrex client request"),
	)
	if err != nil {
		return nil, err
	}
	return &Metrics{
		totalRequestCounter: totalRequestCounter,
		requestDuration:     requestDuration,
	}, nil
}

func InitServerMetrics() (*Metrics, error) {
	totalRequestCounter, err := meter.Int64Counter(
		"shrex_server_total_responses",
		metric.WithDescription("Total count of sent shrex responses"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		"shrex_server_request_duration",
		metric.WithDescription("Time taken to handle a shrex client request"),
	)
	if err != nil {
		return nil, err
	}
	return &Metrics{
		totalRequestCounter: totalRequestCounter,
		requestDuration:     requestDuration,
	}, nil
}
