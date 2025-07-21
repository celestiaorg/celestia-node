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
	// statuses used by the client
	statusSendReqErr  status = "send_req_err"
	statusReadRespErr status = "read_resp_err"
	statusRateLimited status = "rate_limited"
	statusTimeout     status = "timeout"

	// statuses used by the server
	statusReadReqErr  status = "read_req_err"
	statusBadRequest  status = "bad_request"
	statusSendRespErr status = "send_resp_err"

	// general statusese that is applied to both the client and the server
	statusSuccess     status = "success"
	statusNotFound    status = "not_found"
	statusInternalErr status = "internal_err"
)

type Metrics struct {
	totalRequestCounter metric.Int64Counter
	rateLimiterCounter  metric.Int64Counter
	requestDuration     metric.Float64Histogram
}

// observeRequests increments the total number of requests sent with the given status as an
// attribute.
func (m *Metrics) observeRequests(
	ctx context.Context,
	count int64,
	requestName string,
	status status,
	duration time.Duration,
) {
	if m == nil {
		return
	}
	ctx = utils.ResetContextOnError(ctx)
	m.totalRequestCounter.Add(ctx, count,
		metric.WithAttributes(
			attribute.String("request.name", requestName),
			attribute.String("status", string(status)),
		))
	m.requestDuration.Record(ctx, duration.Seconds(),
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

	rateLimiter, err := meter.Int64Counter("shrex_rate_limit_counter",
		metric.WithDescription("concurrency limit of the shrex server"),
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
		rateLimiterCounter:  rateLimiter,
		requestDuration:     requestDuration,
	}, nil
}
