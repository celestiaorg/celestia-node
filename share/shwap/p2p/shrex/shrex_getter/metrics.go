package shrex_getter

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

type requestType int

var (
	edsRequest           requestType = 1
	namespaceDataRequest requestType = 2
	rowRequest           requestType = 3
	samplesRequest       requestType = 4
	rangeRequest         requestType = 5
)

type metrics struct {
	requestAttempt metric.Int64Histogram
}

func (r requestType) String() string {
	switch r {
	case edsRequest:
		return "edsRequest"
	case namespaceDataRequest:
		return "namespaceDataRequest"
	case rowRequest:
		return "rowRequest"
	case samplesRequest:
		return "samplesRequest"
	case rangeRequest:
		return "rangeRequest"
	default:
		panic(fmt.Sprintf("unknown shrex getter requestType: %v", int(r)))
	}
}

func (sg *Getter) WithMetrics() error {
	requestAttempt, err := meter.Int64Histogram(
		"getters_shrex_attempts_per_request",
		metric.WithDescription("Number of attempts per shrex request"),
	)
	if err != nil {
		return err
	}

	sg.metrics = &metrics{
		requestAttempt: requestAttempt,
	}
	return nil
}

func (m *metrics) recordAttempts(ctx context.Context, requestType requestType, attempt int, success bool) {
	if m == nil {
		return
	}
	ctx = utils.ResetContextOnError(ctx)

	m.requestAttempt.Record(ctx, int64(attempt),
		metric.WithAttributes(
			attribute.String("request_type", requestType.String()),
			attribute.Bool("success", success)),
	)
}
