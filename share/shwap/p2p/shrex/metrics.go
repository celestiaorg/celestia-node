package shrex

import (
	"context"

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
}

// ObserveRequests increments the total number of requests sent with the given status as an
// attribute.
func (m *Metrics) ObserveRequests(ctx context.Context, count int64, status status) {
	if m == nil {
		return
	}
	ctx = utils.ResetContextOnError(ctx)
	m.totalRequestCounter.Add(ctx, count,
		metric.WithAttributes(
			attribute.String("status", string(status)),
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

	return &Metrics{
		totalRequestCounter: totalRequestCounter,
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

	return &Metrics{
		totalRequestCounter: totalRequestCounter,
	}, nil
}
