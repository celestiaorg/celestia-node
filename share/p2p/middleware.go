package p2p

import (
	"sync/atomic"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/network"
)

var log = logging.Logger("shrex/middleware")

type Middleware struct {
	concurrencyLimit int64
	parallelRequests int64

	NumRequests    int64
	NumRateLimited int64
}

func NewMiddleware(concurrencyLimit int) *Middleware {
	return &Middleware{
		concurrencyLimit: int64(concurrencyLimit),
	}
}

func (m *Middleware) RateLimitHandler(handler network.StreamHandler) network.StreamHandler {
	return func(stream network.Stream) {
		m.NumRequests++
		current := atomic.AddInt64(&m.parallelRequests, 1)
		defer atomic.AddInt64(&m.parallelRequests, -1)

		if current > m.concurrencyLimit {
			m.NumRateLimited++
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
