package p2p

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	logging "github.com/ipfs/go-log/v2"
)

// serverLogInterceptor logs server request and response
func serverLogInterceptor(logger *logging.ZapEventLogger) ServerInterceptor {
	return func(ctx context.Context, session *Session, handler Handle) error {
		start := time.Now()
		log.Infow("incoming request", "protocol_id", session.Stream.ID())
		err := handler(ctx, session)
		if err != nil {
			logger.Errorw("handling request",
				"protocol_id", session.Stream.ID(),
				"time", time.Since(start))
			return err
		}
		log.Infow("processed request",
			"protocol_id", session.Stream.ID(),
			"time", time.Since(start))
		return nil
	}
}

// clientRetryInterceptor retries request to another peer
func clientRetryInterceptor(perPeerLimit, totalLimit int) ClientInterceptor {
	return func(ctx context.Context, fn performFn, pid protocol.ID, peers ...peer.ID) error {
		var t int
		var err error
		for _, peer := range peers {
			for i := 0; i < perPeerLimit; i++ {
				t++
				err = fn(ctx, pid, peer)
				if err == nil || t == totalLimit {
					return err
				}
			}

		}
		return err
	}
}
