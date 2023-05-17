package core

import (
	"fmt"

	retryhttp "github.com/hashicorp/go-retryablehttp"
	logging "github.com/ipfs/go-log/v2"
	tmlog "github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/client/http"
	"go.uber.org/zap"
)

// Client is an alias to Core Client.
type Client = client.Client

// NewRemote creates a new Client that communicates with a remote Core endpoint over HTTP.
func NewRemote(ip, port string) (Client, error) {
	httpClient := retryhttp.NewClient()
	httpClient.RetryMax = 2
	// suppress logging
	httpClient.Logger = nil

	client, err := http.NewWithClient(
		fmt.Sprintf("tcp://%s:%s", ip, port),
		"/websocket",
		httpClient.StandardClient(),
	)
	if err != nil {
		return nil, err
	}

	client.BaseService = *service.NewBaseService(
		clientLog,
		"core-ws",
		client.WSEvents,
	)
	return client, nil
}

var clientLog = newServiceLogger("core-client")

type serviceLogger struct {
	log *zap.SugaredLogger
}

func newServiceLogger(system string) *serviceLogger {
	return &serviceLogger{&logging.Logger(system).SugaredLogger}
}

func (s *serviceLogger) Debug(msg string, keyvals ...interface{}) {
	s.log.Debugw(msg, keyvals)
}

func (s *serviceLogger) Info(msg string, keyvals ...interface{}) {
	s.log.Infow(msg, keyvals)
}

func (s *serviceLogger) Error(msg string, keyvals ...interface{}) {
	s.log.Errorw(msg, keyvals)
}

func (s *serviceLogger) With(keyvals ...interface{}) tmlog.Logger {
	return &serviceLogger{s.log.With(keyvals)}
}
