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

	host       host.Host
	protocolID protocol.ID

	handler network.StreamHandler
	store   *store.Store

	params  *Parameters
	metrics *shrex.Metrics
}

// NewServer creates new Server
func NewServer(params *Parameters, host host.Host, store *store.Store) (*Server, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-nd: server creation failed: %w", err)
	}

	srv := &Server{
		store:      store,
		host:       host,
		params:     params,
		protocolID: shrex.ProtocolID(params.NetworkID(), protocolString),
	}

	ctx, cancel := context.WithCancel(context.Background())
	srv.cancel = cancel

	handler := srv.streamHandler(ctx)
	withRecovery := shrex.RecoveryMiddleware(handler)
	srv.handler = withRecovery
	return srv, nil
}

// Start starts the server
func (srv *Server) Start(context.Context) error {
	srv.host.SetStreamHandler(srv.protocolID, srv.handler)
	return nil
}

// Stop stops the server
func (srv *Server) Stop(context.Context) error {
	srv.cancel()
	srv.host.RemoveStreamHandler(srv.protocolID)
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

func (srv *Server) handleNamespaceData(ctx context.Context, stream network.Stream) error {
	logger := log.With("source", "server", "peer", stream.Conn().RemotePeer().String())
	logger.Debug("handling nd request")

	startTime := time.Now()
	ndid, err := srv.readRequest(logger, stream)
	if err != nil {
		logger.Warnw("read request", "err", err)
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusBadRequest)
		return err
	}
	fmt.Println("\n\n time to read request (L106):   ", time.Since(startTime).Milliseconds())

	logger = logger.With(
		"namespace", ndid.DataNamespace.String(),
		"height", ndid.Height,
	)
	logger.Debugw("new request")

	ctx, cancel := context.WithTimeout(ctx, srv.params.HandleRequestTimeout)
	defer cancel()

	startTime = time.Now()
	nd, status, err := srv.getNamespaceData(ctx, ndid)
	fmt.Println("\n\n time to get namespaced data (L116):   ", time.Since(startTime).Milliseconds())

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

	startTime = time.Now()
	err = srv.respondStatus(ctx, logger, stream, status)
	fmt.Println("\n\n time to respond status (L131):   ", time.Since(startTime).Milliseconds())

	if err != nil {
		logger.Errorw("sending response", "err", err)
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusSendRespErr)
		return err
	}

	startTime = time.Now()
	_, err = nd.WriteTo(stream)
	fmt.Println("\n\n time to write nd to stream (L141):   ", time.Since(startTime).Milliseconds())

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
	startTime := time.Now()
	file, err := srv.store.GetByHeight(ctx, id.Height)
	if errors.Is(err, store.ErrNotFound) {
		return nil, shrexpb.Status_NOT_FOUND, nil
	}
	fmt.Println("INSIDE GET ND OP: store.GetByHeight took:  ", time.Since(startTime).Milliseconds())
	if err != nil {
		return nil, shrexpb.Status_INTERNAL, fmt.Errorf("retrieving DAH: %w", err)
	}
	defer utils.CloseAndLog(log, "file", file)

	startTime = time.Now()
	nd, err := eds.NamespaceData(ctx, file, id.DataNamespace)

	fmt.Println("INSIDE GET ND OP: eds.NamespaceData took:  ", time.Since(startTime).Milliseconds())
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
