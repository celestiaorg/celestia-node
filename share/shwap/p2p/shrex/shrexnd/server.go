package shrexnd

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
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
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
	middleware *shrex.Middleware
	metrics    *shrex.Metrics
}

// NewServer creates new Server
func NewServer(params *Parameters, host host.Host, store *store.Store, supportedProtocols []string) (*Server, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-server: Server creation failed: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		ctx:                ctx,
		cancel:             cancel,
		store:              store,
		host:               host,
		params:             params,
		supportedProtocols: supportedProtocols,
		middleware:         shrex.NewMiddleware(params.ConcurrencyLimit),
	}, nil
}

// Start starts the Server
func (srv *Server) Start(context.Context) error {
	for _, protocolName := range srv.supportedProtocols {
		if _, ok := initID[protocolName]; !ok {
			return fmt.Errorf("shrex-server: %w: %s", shrex.ErrUnsupportedProtocol,
				shrex.ProtocolID(srv.params.NetworkID(), protocolName),
			)
		}

		handler := srv.streamHandler(srv.ctx, protocolName)
		withRateLimit := srv.middleware.RateLimitHandler(handler)
		withRecovery := shrex.RecoveryMiddleware(withRateLimit)
		srv.SetHandler(shrex.ProtocolID(srv.params.NetworkID(), protocolName), withRecovery)
	}
	return nil
}

func (srv *Server) SetHandler(p protocol.ID, h network.StreamHandler) {
	srv.host.SetStreamHandler(p, h)
}

// Stop stops the Server
func (srv *Server) Stop(context.Context) error {
	srv.cancel()
	for _, id := range srv.supportedProtocols {
		srv.host.RemoveStreamHandler(shrex.ProtocolID(srv.params.NetworkID(), id))
	}
	return nil
}

func (srv *Server) streamHandler(ctx context.Context, idName string) network.StreamHandler {
	return func(s network.Stream) {
		err := srv.handleDataRequest(ctx, idName, s)
		if err != nil {
			s.Reset() //nolint:errcheck
		}
		if err = s.Close(); err != nil {
			log.Debugw("shrex-server: closing stream", "err", err)
		}
	}
}

func (srv *Server) handleDataRequest(ctx context.Context, idName string, stream network.Stream) error {
	logger := log.With("source", "Server", "peer", stream.Conn().RemotePeer().String())
	logger.Debug("handling data request")

	srv.obServeRateLimitedRequests()

	// registering handlers are done only after the verification that
	// protocol supports it. There is no need to additionally verify here whether we support it
	// or not.
	requestIDFn := initID[idName]
	requestID := requestIDFn()

	err := srv.readRequest(logger, requestID, stream)
	if err != nil {
		logger.Warnw("read request", "err", err)
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusBadRequest)
		return err
	}

	err = requestID.Validate()
	if err != nil {
		logger.Warnw("validate request", "err", err)
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusBadRequest)
		return err
	}

	logger.Debugw("new request")
	ctx, cancel := context.WithTimeout(ctx, srv.params.HandleRequestTimeout)
	defer cancel()

	r, status, err := srv.getData(ctx, requestID)

	sendErr := srv.respondStatus(ctx, logger, stream, status)
	if sendErr != nil {
		logger.Errorw("sending response status", "err", sendErr)
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusSendRespErr)
	}
	if err != nil {
		logger.Errorw("handling request", "err", err)
		return errors.Join(err, sendErr)
	}

	if status != shrexpb.Status_OK {
		return nil
	}

	_, err = io.Copy(stream, r)
	if err != nil {
		logger.Errorw("send data", "err", err)
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusSendRespErr)
	}
	return err
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
	file, err := srv.store.GetByHeight(ctx, id.Target())
	switch {
	case err == nil:
	case errors.Is(err, store.ErrNotFound):
		return nil, shrexpb.Status_NOT_FOUND, nil
	default:
		return nil, shrexpb.Status_INTERNAL, fmt.Errorf("retrieving DAH: %w", err)
	}

	defer utils.CloseAndLog(log, "file", file)

	w, err := id.FetchContainerReader(ctx, file)
	if err != nil {
		return nil, shrexpb.Status_INVALID, fmt.Errorf("getting data: %w", err)
	}
	return w, shrexpb.Status_OK, nil
}

func (srv *Server) obServeRateLimitedRequests() {
	numRateLimited := srv.middleware.DrainCounter()
	if numRateLimited > 0 {
		srv.metrics.ObserveRequests(context.Background(), numRateLimited, shrex.StatusRateLimited)
	}
}

func (srv *Server) respondStatus(
	ctx context.Context,
	logger *zap.SugaredLogger,
	stream network.Stream,
	status shrexpb.Status,
) error {
	srv.observeStatus(ctx, status)

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

func (srv *Server) observeStatus(ctx context.Context, status shrexpb.Status) {
	switch {
	case status == shrexpb.Status_OK:
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusSuccess)
	case status == shrexpb.Status_NOT_FOUND:
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusNotFound)
	case status == shrexpb.Status_INVALID:
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusInternalErr)
	}
}
