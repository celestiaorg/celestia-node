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

	store *store.Store

	params *ServerParams
	// TODO: decouple middleware metrics from shrex and remove middleware from Server
	middleware *Middleware
	metrics    *Metrics
}

// NewServer creates a new shrEx-Server. It configures the server with the provided
// parameters, host, and data store. By default, it creates handlers for all types
// of the requests that the node supports unless the user specifies what protocols should be enabled.
func NewServer(
	params *ServerParams,
	host host.Host,
	store *store.Store,
) (*Server, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex/server: parameters are not valid: %w", err)
	}

	middleware, err := newMiddleware(params.ConcurrencyLimit)
	if err != nil {
		return nil, fmt.Errorf("shrex/server: could not initialize middleware: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	srv := &Server{
		ctx:        ctx,
		cancel:     cancel,
		store:      store,
		host:       host,
		middleware: middleware,
		params:     params,
	}
	return srv, nil
}

func (srv *Server) SetHandler(p protocol.ID, h network.StreamHandler) {
	srv.host.SetStreamHandler(p, h)
}

func (srv *Server) Start(_ context.Context) error {
	for _, reqID := range registry {
		id := reqID()
		handler := srv.streamHandler(srv.ctx, reqID)
		withRateLimit := srv.middleware.rateLimitHandler(srv.ctx, handler, srv.metrics, id.Name())
		withRecovery := RecoveryMiddleware(withRateLimit)

		p := ProtocolID(srv.params.NetworkID(), id.Name())

		log.Info("shrex/server: set handler for: ", p)

		srv.SetHandler(p, withRecovery)
	}
	return nil
}

// Stop stops the Server
func (srv *Server) Stop(_ context.Context) error {
	srv.cancel()
	for _, reqID := range registry {
		srv.host.RemoveStreamHandler(ProtocolID(srv.params.NetworkID(), reqID().Name()))
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

func (srv *Server) streamHandler(ctx context.Context, id newRequestID) network.StreamHandler {
	return func(s network.Stream) {
		requestID := id()
		handleTime := time.Now()
		status := srv.handleDataRequest(ctx, requestID, s)
		srv.metrics.observeRequests(ctx, 1, requestID.Name(), status, time.Since(handleTime))
		log.Debugw("server: handling request",
			"name", requestID.Name(),
			"status", status,
			"duration", time.Since(handleTime),
		)
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
	log.Debugf("server: handling data request: %s from peer: %s", requestID.Name(), stream.Conn().RemotePeer())

	err := stream.SetReadDeadline(time.Now().Add(srv.params.ReadTimeout))
	if err != nil {
		log.Debugw("server: setting read deadline", "err", err)
	}

	_, err = requestID.ReadFrom(stream)
	if err != nil {
		log.Errorf("server: reading request %s from peer %s, %w", requestID.Name(), stream.Conn().RemotePeer(), err)
		return statusReadReqErr
	}

	logger := log.With(
		"source", "server",
		"name", requestID.Name(),
		"height", requestID.Height(),
		"peer", stream.Conn().RemotePeer().String(),
	)

	err = stream.CloseRead()
	if err != nil {
		log.Warnw("server: closing read side of the stream", "err", err)
	}

	err = requestID.Validate()
	if err != nil {
		logger.Warnw("validate request", "err", err)
		return statusBadRequest
	}

	ctx, cancel := context.WithTimeout(ctx, srv.params.HandleRequestTimeout)
	defer cancel()

	file, err := srv.store.GetByHeight(ctx, requestID.Height())

	deadlineErr := stream.SetWriteDeadline(time.Now().Add(srv.params.WriteTimeout))
	if deadlineErr != nil {
		log.Debugw("server: setting write deadline", "err", err)
	}

	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			logger.Errorf("file not found in store")
			return respondStatus(logger, shrexpb.Status_NOT_FOUND, stream)
		}
		logger.Errorf("getting header %w", err)
		return respondStatus(logger, shrexpb.Status_INTERNAL, stream)
	}

	defer utils.CloseAndLog(log, "file", file)
	r, err := requestID.ResponseReader(ctx, file)
	if err != nil {
		logger.Errorf("getting data from response reader %w", err)
		return respondStatus(logger, shrexpb.Status_INTERNAL, stream)
	}

	status := respondStatus(logger, shrexpb.Status_OK, stream)
	logger.Debugw("sending status", "status", status)
	if status != statusSuccess {
		return status
	}

	_, err = io.Copy(stream, r)
	if err != nil {
		logger.Errorw("send data", "err", err)
		return statusSendRespErr
	}
	logger.Debugw("sent the data to the client")
	return statusSuccess
}

func respondStatus(log *zap.SugaredLogger, status shrexpb.Status, stream network.Stream) status {
	_, err := serde.Write(stream, &shrexpb.Response{Status: status})
	if err != nil {
		log.Errorw("sending response status", "err", err)
		return statusSendStatusErr
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
