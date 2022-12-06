package p2p

import logging "github.com/ipfs/go-log/v2"

func logIntercepter(logger *logging.ZapEventLogger) Interceptor {
	return func(session *Session, handle Handle) error {
		log.Info("incoming request", "protocol_id", session.Stream.ID())
		err := handle(session)
		if err != nil {
			logger.Errorw("handling request", "protocol_id", session.Stream.ID())
			return err
		}
		log.Info("processed request", "protocol_id", session.Stream.ID())
		return nil
	}
}
