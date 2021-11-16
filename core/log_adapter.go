package core

import (
	"go.uber.org/zap"

	logging "github.com/ipfs/go-log/v2"

	tmlog "github.com/tendermint/tendermint/libs/log"
)

var log = logging.Logger("core")

// logAdapter is needed to push through our logger to celestia-core
type logAdapter struct {
	log *zap.SugaredLogger
}

func adaptedLogger() *logAdapter {
	return &logAdapter{
		log: &log.SugaredLogger,
	}
}

func (l *logAdapter) Debug(msg string, keyVals ...interface{}) {
	l.log.Debugw(msg, keyVals...)
}

func (l *logAdapter) Info(msg string, keyVals ...interface{}) {
	l.log.Infow(msg, keyVals...)
}

func (l *logAdapter) Error(msg string, keyVals ...interface{}) {
	l.log.Errorw(msg, keyVals...)
}

func (l *logAdapter) With(keyVals ...interface{}) tmlog.Logger {
	return &logAdapter{l.log.With(keyVals...)}
}
