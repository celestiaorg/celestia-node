package shrex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/zap"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	"github.com/celestiaorg/celestia-node/libs/utils"
	shrexpb "github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/pb"
	"github.com/celestiaorg/celestia-node/store"
)

// Server implements Server side of shrex protocol to serve data to remote
// peers.
type Server struct {
	ctx    context.Context
	cancel context.CancelFunc

	host host.Host
	// list of protocol names that is supported by the server
	enabledProtocols []string

	store *store.Store

	params *ServerParams
	// TODO: decouple middleware metrics from shrex and remove middleware from Server
	middleware *Middleware
	metrics    *Metrics
}

// NewServer creates a new shrEx-Server. It configures the server with the provided
// parameters, host, and data store. By default, it creates handlers for all types
// of the requests that the node supports unless the user specifies what protocols should be enabled.
func NewServer(params *ServerParams, host host.Host, store *store.Store, enabledProtocols ...string) (*Server, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex/server: parameters are not valid: %w", err)
	}

	middleware, err := NewMiddleware(params.ConcurrencyLimit)
	if err != nil {
		return nil, fmt.Errorf("shrex/server: could not initialize middleware: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	srv := &Server{
		ctx:              ctx,
		cancel:           cancel,
		store:            store,
		host:             host,
		params:           params,
		enabledProtocols: SupportedProtocols(),
		middleware:       middleware,
	}
	if enabledProtocols != nil {
		// overwrite the default protocols if they were provided by the user
		srv.enabledProtocols = enabledProtocols
	}

	initID := newRequestID()
	for _, protocolName := range enabledProtocols {
		id, ok := initID[protocolName]
		if !ok {
			return nil, fmt.Errorf("shrex/server: %w: %s", ErrUnsupportedProtocol,
				ProtocolID(params.NetworkID(), protocolName),
			)
		}
		handler := srv.streamHandler(srv.ctx, id)
		withRateLimit := srv.middleware.rateLimitHandler(ctx, handler, id().Name())
		withRecovery := RecoveryMiddleware(withRateLimit)
		srv.SetHandler(ProtocolID(srv.params.NetworkID(), protocolName), withRecovery)
	}
	return srv, nil
}

func (srv *Server) SetHandler(p protocol.ID, h network.StreamHandler) {
	srv.host.SetStreamHandler(p, h)
}

// Stop stops the Server
func (srv *Server) Stop(_ context.Context) error {
	srv.cancel()
	for _, id := range srv.enabledProtocols {
		srv.host.RemoveStreamHandler(ProtocolID(srv.params.NetworkID(), id))
	}
	return nil
}

func (srv *Server) WithMetrics() error {
	metrics, err := InitServerMetrics()
	if err != nil {
		return fmt.Errorf("shrex/server: init Metrics: %w", err)
	}
	srv.metrics = metrics
	return nil
}

func (srv *Server) streamHandler(ctx context.Context, id newRequest) network.StreamHandler {
	return func(s network.Stream) {
		requestID := id()
		handleTime := time.Now()
		status := srv.handleDataRequest(ctx, requestID, s)
		srv.metrics.observeRequests(ctx, 1, requestID.Name(), status, time.Since(handleTime))
		// reset because we will not send anything back
		if status == statusBadRequest || status == statusReadReqErr {
			s.Reset() //nolint:errcheck
			return
		}

		if err := s.Close(); err != nil {
			log.Debugw("server: closing stream", "err", err)
		}
	}
}

func (srv *Server) handleDataRequest(ctx context.Context, requestID request, stream network.Stream) status {
	logger := log.With(
		"source", "server",
		"name", requestID.Name(),
		"peer", stream.Conn().RemotePeer().String(),
	)

	logger.Debugf("handling data request: %s", requestID.Name())

	err := stream.SetReadDeadline(time.Now().Add(srv.params.ReadTimeout))
	if err != nil {
		logger.Debugw("setting read deadline", "err", err)
	}

	_, err = requestID.ReadFrom(stream)
	if err != nil {
		logger.Warnf("reading request: %w", err)
		return statusReadReqErr
	}

	err = stream.CloseRead()
	if err != nil {
		logger.Warnw("closing read side of the stream", "err", err)
	}

	err = requestID.Validate()
	if err != nil {
		logger.Warnw("validate request", "err", err)
		return statusBadRequest
	}

	logger.Debugw("new request")
	ctx, cancel := context.WithTimeout(ctx, srv.params.HandleRequestTimeout)
	defer cancel()

	var r io.Reader
	file, err := srv.store.GetByHeight(ctx, requestID.Height())
	if err == nil {
		defer utils.CloseAndLog(log, "file", file)
		r, err = requestID.ContainerDataReader(ctx, file)
	}

	deadlineErr := stream.SetWriteDeadline(time.Now().Add(srv.params.WriteTimeout))
	if deadlineErr != nil {
		logger.Debugw("setting write deadline", "err", err)
	}

	switch {
	case errors.Is(err, store.ErrNotFound):
		logger.Errorf("file not found")
		return respondStatus(logger, shrexpb.Status_NOT_FOUND, stream)
	case err != nil:
		logger.Errorf("getting the data %w", err)
		return respondStatus(logger, shrexpb.Status_INTERNAL, stream)
	}

	status := respondStatus(logger, shrexpb.Status_OK, stream)
	if status != statusSuccess {
		return status
	}

	_, err = io.Copy(stream, r)
	if err != nil {
		logger.Errorw("send data", "err", err)
		return statusSendRespErr
	}
	return statusSuccess
}

func respondStatus(log *zap.SugaredLogger, status shrexpb.Status, stream network.Stream) status {
	_, err := serde.Write(stream, &shrexpb.Response{Status: status})
	if err != nil {
		log.Errorw("sending response status", "err", err)
		return statusSendRespErr
	}

	switch status {
	case shrexpb.Status_INTERNAL:
		return statusInternalErr
	case shrexpb.Status_NOT_FOUND:
		return statusNotFound
	case shrexpb.Status_OK:
		return statusSuccess
	default:
		panic("unknown status")
	}
}
