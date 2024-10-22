package shrex

import (
	"sync/atomic"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/network"
)

var log = logging.Logger("shrex/middleware")

type Middleware struct {
	// concurrencyLimit is the maximum number of requests that can be processed at once.
	concurrencyLimit int64
	// parallelRequests is the number of requests currently being processed.
	parallelRequests atomic.Int64
	// numRateLimited is the number of requests that were rate limited.
	numRateLimited atomic.Int64
}

func NewMiddleware(concurrencyLimit int) *Middleware {
	return &Middleware{
		concurrencyLimit: int64(concurrencyLimit),
	}
}

// DrainCounter returns the current value of the rate limit counter and resets it to 0.
func (m *Middleware) DrainCounter() int64 {
	return m.numRateLimited.Swap(0)
}

func (m *Middleware) RateLimitHandler(handler network.StreamHandler) network.StreamHandler {
	return func(stream network.Stream) {
		current := m.parallelRequests.Add(1)
		defer m.parallelRequests.Add(-1)

		if current > m.concurrencyLimit {
			m.numRateLimited.Add(1)
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
