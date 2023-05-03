package p2p

import (
	"sync/atomic"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/network"
)

var log = logging.Logger("shrex/middleware")

type Middleware struct {
	// NumRateLimited is the number of requests that were rate limited. It is reset to 0 every time
	// it is read and observed into metrics.
	NumRateLimited atomic.Int64

	concurrencyLimit int64
	parallelRequests atomic.Int64
}

func NewMiddleware(concurrencyLimit int) *Middleware {
	return &Middleware{
		concurrencyLimit: int64(concurrencyLimit),
	}
}

func (m *Middleware) RateLimitHandler(handler network.StreamHandler) network.StreamHandler {
	return func(stream network.Stream) {
		current := m.parallelRequests.Add(1)
		defer m.parallelRequests.Add(-1)

		if current > m.concurrencyLimit {
			m.NumRateLimited.Add(1)
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
