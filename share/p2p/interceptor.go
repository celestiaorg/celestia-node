package p2p

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
)

func logIntercepter(logger *logging.ZapEventLogger) ServerInterceptor {
	return func(ctx context.Context, session *Session, handler Handle) error {
		log.Info("incoming request", "protocol_id", session.Stream.ID())
		err := handler(ctx, session)
		if err != nil {
			logger.Errorw("handling request", "protocol_id", session.Stream.ID())
			return err
		}
		log.Info("processed request", "protocol_id", session.Stream.ID())
		return nil
	}
}
