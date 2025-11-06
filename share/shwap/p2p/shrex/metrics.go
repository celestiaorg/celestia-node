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
	statusOpenStreamErr status = "open_stream_err"
	statusSendReqErr    status = "send_req_err"
	statusReadStatusErr status = "read_status_err"
	statusReadRespErr   status = "read_resp_err"

	// statuses used by the server
	statusReadReqErr    status = "read_req_err"
	statusBadRequest    status = "bad_request"
	statusSendStatusErr status = "send_status_err"
	statusSendRespErr   status = "send_resp_err"

	// general statuses that are applied to both the client and the server
	statusSuccess     status = "success"
	statusNotFound    status = "not_found"
	statusInternalErr status = "internal_err"
)

type Metrics struct {
	totalRequestCounter metric.Int64Counter
	requestDuration     metric.Float64Histogram
	// payloadServed will aggregate the total payload served
	// by the shrex server
	payloadServed metric.Int64Counter
}

// observeRequest increments the total number of requests sent with the given status as an
// attribute.
func (m *Metrics) observeRequest(
	ctx context.Context,
	requestName string,
	status status,
	duration time.Duration,
) {
	if m == nil {
		return
	}

	ctx = utils.ResetContextOnError(ctx)

	opt := metric.WithAttributes(
		attribute.String("protocol", requestName),
		attribute.String("status", string(status)),
	)

	m.totalRequestCounter.Add(ctx, 1, opt)
	m.requestDuration.Record(ctx, duration.Seconds(), opt)
}

// observePayloadServed records the size of the payload served by the shrex
// server for successful requests only
func (m *Metrics) observePayloadServed(
	ctx context.Context,
	requestName string,
	status status,
	payloadSize int,
) {
	if m == nil || payloadSize == 0 {
		return
	}

	// if the request was unsuccessful, assume it's a partial response
	// this attribute can be used to filter successful throughput vs total
	incomplete := status != statusSuccess

	m.payloadServed.Add(ctx, int64(payloadSize), metric.WithAttributes(
		attribute.String("protocol", requestName),
		attribute.Bool("incomplete", incomplete),
	))
}

func InitClientMetrics() (*Metrics, error) {
	totalRequestCounter, err := meter.Int64Counter(
		"shrex_server_total_responses",
		metric.WithDescription("Total count of sent shrex responses"),
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
		requestDuration:     requestDuration,
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

	requestDuration, err := meter.Float64Histogram(
		"shrex_server_request_duration",
		metric.WithDescription("Time taken to handle a shrex client request"),
	)
	if err != nil {
		return nil, err
	}

	payloadServedHist, err := meter.Int64Counter(
		"shrex_payload_served_bytes",
		metric.WithDescription("Total request data served by shrex server in bytes"),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		totalRequestCounter: totalRequestCounter,
		requestDuration:     requestDuration,
		payloadServed:       payloadServedHist,
	}, nil
}
