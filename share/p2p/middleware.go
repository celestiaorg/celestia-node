package p2p

import (
	"sync/atomic"

	logging "github.com/ipfs/go-log/v2"

	"github.com/libp2p/go-libp2p/core/network"
)

var log = logging.Logger("shrex/middleware")

func RateLimitMiddleware(inner network.StreamHandler, concurrencyLimit int) network.StreamHandler {
	var parallelRequests int64
	limit := int64(concurrencyLimit)
	return func(stream network.Stream) {
		current := atomic.AddInt64(&parallelRequests, 1)
		defer atomic.AddInt64(&parallelRequests, -1)

		if current > limit {
			log.Debug("concurrency limit reached")
			err := stream.Close()
			if err != nil {
				log.Errorw("server: closing stream", "err", err)
			}
			return
		}
		inner(stream)
	}
}
