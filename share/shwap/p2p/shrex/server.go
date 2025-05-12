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

	host               host.Host
	supportedProtocols []string

	store *store.Store

	params *Parameters
	// TODO: decouple middleware metrics from shrex and remove middleware from Server
	middleware *Middleware
	metrics    *Metrics
}

// NewServer creates new Server
func NewServer(params *Parameters, host host.Host, store *store.Store, supportedProtocols ...string) (*Server, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex/server: Server creation failed: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	srv := &Server{
		ctx:                ctx,
		cancel:             cancel,
		store:              store,
		host:               host,
		params:             params,
		supportedProtocols: SupportedProtocols(),
		middleware:         NewMiddleware(params.ConcurrencyLimit),
	}
	if supportedProtocols != nil {
		// overwrite the default protocols if they were provided by the user
		srv.supportedProtocols = supportedProtocols
	}

	initID := newInitID()
	for _, protocolName := range supportedProtocols {
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
	for _, id := range srv.supportedProtocols {
		srv.host.RemoveStreamHandler(ProtocolID(srv.params.NetworkID(), id))
	}
	return nil
}

func (srv *Server) streamHandler(ctx context.Context, id newID) network.StreamHandler {
	return func(s network.Stream) {
		if !srv.handleDataRequest(ctx, id, s) {
			s.Reset() //nolint:errcheck
		}
		if err := s.Close(); err != nil {
			log.Debugw("server: closing stream", "err", err)
		}
	}
}

func (srv *Server) handleDataRequest(ctx context.Context, id newID, stream network.Stream) bool {
	requestID := id()
	logger := log.With("source", "Server", "name", "peer", requestID.Name(), stream.Conn().RemotePeer().String())
	logger.Debug("handling data request")

	// registering handlers are done only after the verification that
	// protocol supports it. There is no need to additionally verify here whether we support it
	// or not.

	err := srv.readRequest(logger, requestID, stream)
	if err != nil {
		logger.Warnw("read request", "err", err)
		srv.metrics.observeRequests(ctx, 1, requestID.Name(), StatusBadRequest)
		return false
	}

	err = requestID.Validate()
	if err != nil {
		logger.Warnw("validate request", "err", err)
		srv.metrics.observeRequests(ctx, 1, requestID.Name(), StatusBadRequest)
		return false
	}

	logger.Debugw("new request")
	ctx, cancel := context.WithTimeout(ctx, srv.params.HandleRequestTimeout)
	defer cancel()

	r, status, err := srv.getData(ctx, requestID)
	sendErr := srv.respondStatus(ctx, logger, requestID, stream, status)
	if sendErr != nil {
		logger.Errorw("sending response status", "err", sendErr)
		srv.metrics.observeRequests(ctx, 1, requestID.Name(), StatusSendRespErr)
	}

	if err != nil {
		logger.Errorw("handling request", "err", errors.Join(err, sendErr))
		return false
	}

	if status != shrexpb.Status_OK {
		return false
	}

	_, err = io.Copy(stream, r)
	if err != nil {
		logger.Errorw("send data", "err", err)
		srv.metrics.observeRequests(ctx, 1, requestID.Name(), StatusSendRespErr)
		return false
	}
	return true
}

func (srv *Server) readRequest(
	logger *zap.SugaredLogger,
	id id,
	stream network.Stream,
) error {
	err := stream.SetReadDeadline(time.Now().Add(srv.params.ServerReadTimeout))
	if err != nil {
		logger.Debugw("setting read deadline", "err", err)
	}

	_, err = id.ReadFrom(stream)
	if err != nil {
		return fmt.Errorf("reading request: %w", err)
	}

	err = stream.CloseRead()
	if err != nil {
		logger.Warnw("closing read side of the stream", "err", err)
	}

	return nil
}

func (srv *Server) getData(
	ctx context.Context,
	id id,
) (io.Reader, shrexpb.Status, error) {
	handleTime := time.Now()
	file, err := srv.store.GetByHeight(ctx, id.Target())
	switch {
	case err == nil:
	case errors.Is(err, store.ErrNotFound):
		return nil, shrexpb.Status_NOT_FOUND, nil
	default:
		return nil, shrexpb.Status_INTERNAL, fmt.Errorf("retrieving DAH: %w", err)
	}

	defer utils.CloseAndLog(log, "file", file)

	w, err := id.ContainerDataReader(ctx, file)
	if err != nil {
		return nil, shrexpb.Status_INVALID, fmt.Errorf("getting data: %w", err)
	}
	srv.metrics.observeDuration(ctx, id.Name(), time.Since(handleTime))
	return w, shrexpb.Status_OK, nil
}

func (srv *Server) respondStatus(
	ctx context.Context,
	logger *zap.SugaredLogger,
	id id,
	stream network.Stream,
	status shrexpb.Status,
) error {
	srv.observeStatus(ctx, id.Name(), status)

	err := stream.SetWriteDeadline(time.Now().Add(srv.params.ServerWriteTimeout))
	if err != nil {
		logger.Debugw("setting write deadline", "err", err)
	}

	_, err = serde.Write(stream, &shrexpb.Response{Status: status})
	if err != nil {
		return fmt.Errorf("writing response: %w", err)
	}

	return nil
}

func (srv *Server) observeStatus(ctx context.Context, requestName string, status shrexpb.Status) {
	switch {
	case status == shrexpb.Status_OK:
		srv.metrics.observeRequests(ctx, 1, requestName, StatusSuccess)
	case status == shrexpb.Status_NOT_FOUND:
		srv.metrics.observeRequests(ctx, 1, requestName, StatusNotFound)
	case status == shrexpb.Status_INVALID:
		srv.metrics.observeRequests(ctx, 1, requestName, StatusInternalErr)
	}
}

func (srv *Server) WithMetrics() error {
	metrics, err := InitServerMetrics()
	if err != nil {
		return fmt.Errorf("shrex/server: init Metrics: %w", err)
	}
	srv.metrics = metrics
	return nil
}
