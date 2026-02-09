package shrex_getter

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

type metrics struct {
	requestAttempt metric.Int64Histogram
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

func (m *metrics) recordAttempts(ctx context.Context, requestName string, attempt int, success bool) {
	if m == nil {
		return
	}
	ctx = utils.ResetContextOnError(ctx)

	m.requestAttempt.Record(ctx, int64(attempt),
		metric.WithAttributes(
			attribute.String("request_type", requestName),
			attribute.Bool("success", success)),
	)
}
