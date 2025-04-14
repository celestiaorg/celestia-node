package shrexnd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/zap"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	shrexpb "github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/pb"
	"github.com/celestiaorg/celestia-node/store"
)

// Server implements server side of shrex/nd protocol to serve namespaced share to remote
// peers.
type Server struct {
	cancel context.CancelFunc

	host        host.Host
	protocolIDs []protocol.ID

	handler network.StreamHandler
	store   *store.Store

	params *Parameters
	// TODO: decouple middleware metrics from shrex and remove middleware from Server
	middleware *shrex.Middleware
	metrics    *shrex.Metrics
}

// NewServer creates new Server
func NewServer(params *Parameters, host host.Host, store *store.Store) (*Server, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-nd: server creation failed: %w", err)
	}

	var protocolIDs []protocol.ID
	for _, name := range protocolNames {
		protocolIDs = append(protocolIDs, shrex.ProtocolID(params.NetworkID(), name))
	}

	srv := &Server{
		store:       store,
		host:        host,
		params:      params,
		protocolIDs: protocolIDs,
		middleware:  shrex.NewMiddleware(params.ConcurrencyLimit),
	}

	ctx, cancel := context.WithCancel(context.Background())
	srv.cancel = cancel

	handler := srv.streamHandler(ctx)
	withRateLimit := srv.middleware.RateLimitHandler(handler)
	withRecovery := shrex.RecoveryMiddleware(withRateLimit)
	srv.handler = withRecovery
	return srv, nil
}

// Start starts the server
func (srv *Server) Start(context.Context) error {
	for _, id := range srv.protocolIDs {
		srv.host.SetStreamHandler(id, srv.handler)
	}
	return nil
}

// Stop stops the server
func (srv *Server) Stop(context.Context) error {
	srv.cancel()
	for _, id := range srv.protocolIDs {
		srv.host.RemoveStreamHandler(id)
	}
	return nil
}

func (srv *Server) streamHandler(ctx context.Context) network.StreamHandler {
	return func(s network.Stream) {
		err := srv.handleNamespaceData(ctx, s)
		if err != nil {
			s.Reset() //nolint:errcheck
			return
		}
		if err = s.Close(); err != nil {
			log.Debugw("server: closing stream", "err", err)
		}
	}
}

// SetHandler sets server handler
func (srv *Server) SetHandler(handler network.StreamHandler) {
	srv.handler = handler
}

func (srv *Server) observeRateLimitedRequests() {
	numRateLimited := srv.middleware.DrainCounter()
	if numRateLimited > 0 {
		srv.metrics.ObserveRequests(context.Background(), numRateLimited, shrex.StatusRateLimited)
	}
}

func (srv *Server) handleNamespaceData(ctx context.Context, stream network.Stream) error {
	logger := log.With("source", "server", "peer", stream.Conn().RemotePeer().String())
	logger.Debug("handling nd request")

	srv.observeRateLimitedRequests()
	ndid, err := srv.readRequest(logger, stream)
	if err != nil {
		logger.Warnw("read request", "err", err)
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusBadRequest)
		return err
	}

	logger = logger.With(
		"namespace", ndid.DataNamespace.String(),
		"height", ndid.Height,
	)
	logger.Debugw("new request")

	ctx, cancel := context.WithTimeout(ctx, srv.params.HandleRequestTimeout)
	defer cancel()

	nd, status, err := srv.getNamespaceData(ctx, ndid)
	if err != nil {
		// server should respond with status regardless if there was an error getting data
		sendErr := srv.respondStatus(ctx, logger, stream, status)
		if sendErr != nil {
			logger.Errorw("sending response", "err", sendErr)
			srv.metrics.ObserveRequests(ctx, 1, shrex.StatusSendRespErr)
		}
		logger.Errorw("handling request", "err", err)
		return errors.Join(err, sendErr)
	}

	err = srv.respondStatus(ctx, logger, stream, status)
	if err != nil {
		logger.Errorw("sending response", "err", err)
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusSendRespErr)
		return err
	}

	_, err = nd.WriteTo(stream)
	if err != nil {
		logger.Errorw("send nd data", "err", err)
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusSendRespErr)
		return err
	}
	return nil
}

func (srv *Server) readRequest(
	logger *zap.SugaredLogger,
	stream network.Stream,
) (shwap.NamespaceDataID, error) {
	err := stream.SetReadDeadline(time.Now().Add(srv.params.ServerReadTimeout))
	if err != nil {
		logger.Debugw("setting read deadline", "err", err)
	}

	ndid := shwap.NamespaceDataID{}
	_, err = ndid.ReadFrom(stream)
	if err != nil {
		return shwap.NamespaceDataID{}, fmt.Errorf("reading request: %w", err)
	}

	err = stream.CloseRead()
	if err != nil {
		logger.Warnw("closing read side of the stream", "err", err)
	}

	return ndid, nil
}

func (srv *Server) getNamespaceData(
	ctx context.Context,
	id shwap.NamespaceDataID,
) (shwap.NamespaceData, shrexpb.Status, error) {
	file, err := srv.store.GetByHeight(ctx, id.Height)
	if errors.Is(err, store.ErrNotFound) {
		return nil, shrexpb.Status_NOT_FOUND, nil
	}
	if err != nil {
		return nil, shrexpb.Status_INTERNAL, fmt.Errorf("retrieving DAH: %w", err)
	}
	defer utils.CloseAndLog(log, "file", file)

	nd, err := eds.NamespaceData(ctx, file, id.DataNamespace)
	if err != nil {
		return nil, shrexpb.Status_INVALID, fmt.Errorf("getting nd: %w", err)
	}

	return nd, shrexpb.Status_OK, nil
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
	case status != shrexpb.Status_NOT_FOUND:
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusNotFound)
	case status == shrexpb.Status_INVALID:
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusInternalErr)
	}
}

var protocolNames = []string{
	shwap.EDSName,
	shwap.RowName,
	shwap.NamespaceDataName,
	shwap.SampleName,
	// TODO: add more
}
