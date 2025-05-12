package shrex

import (
	"context"
	"sync/atomic"

	"github.com/libp2p/go-libp2p/core/network"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Middleware struct {
	// concurrencyLimit is the maximum number of requests that can be processed at once.
	concurrencyLimit int64
	// parallelRequests is the number of requests currently being processed.
	parallelRequests atomic.Int64

	rateLimiterCounter metric.Int64Counter
}

func NewMiddleware(concurrencyLimit int) *Middleware {
	return &Middleware{
		concurrencyLimit: int64(concurrencyLimit),
	}
}

func (m *Middleware) rateLimitHandler(
	ctx context.Context,
	handler network.StreamHandler,
	requestName string,
) network.StreamHandler {
	return func(stream network.Stream) {
		current := m.parallelRequests.Add(1)
		defer m.parallelRequests.Add(-1)

		if current > m.concurrencyLimit {
			m.rateLimiterCounter.Add(ctx, 1,
				metric.WithAttributes(attribute.String("request.name", requestName)),
			)
			log.Debug("concurrency limit reached")
			err := stream.Close()
			if err != nil {
				log.Debugw("server: closing stream", "err", err)
			}
			return
		}
		handler(stream)
	}
}
