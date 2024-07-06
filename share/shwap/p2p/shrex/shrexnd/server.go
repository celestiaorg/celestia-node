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
	nmt_pb "github.com/celestiaorg/nmt/pb"

	"github.com/celestiaorg/celestia-node/libs/utils"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	pb "github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexnd/pb"
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

	params     *Parameters
	middleware *shrex.Middleware
	metrics    *shrex.Metrics
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
		middleware: shrex.NewMiddleware(params.ConcurrencyLimit),
	}

	ctx, cancel := context.WithCancel(context.Background())
	srv.cancel = cancel

	srv.handler = srv.middleware.RateLimitHandler(srv.streamHandler(ctx))
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
		err := srv.handleNamespacedData(ctx, s)
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

func (srv *Server) handleNamespacedData(ctx context.Context, stream network.Stream) error {
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

	ctx, cancel := context.WithTimeout(ctx, srv.params.HandleRequestTimeout)
	defer cancel()

	shares, status, err := srv.getNamespaceData(ctx, ndid)
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

	err = srv.sendNamespacedShares(shares, stream)
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

	req := make([]byte, shwap.NamespaceDataIDSize)
	_, err = io.ReadFull(stream, req)
	if err != nil {
		return shwap.NamespaceDataID{}, fmt.Errorf("reading request: %w", err)
	}
	id, err := shwap.NamespaceDataIDFromBinary(req)
	if err != nil {
		return shwap.NamespaceDataID{}, fmt.Errorf("decoding request: %w", err)
	}

	logger.Debugw("new request")
	err = stream.CloseRead()
	if err != nil {
		logger.Debugw("closing read side of the stream", "err", err)
	}

	return id, nil
}

func (srv *Server) getNamespaceData(ctx context.Context, id shwap.NamespaceDataID) (shwap.NamespacedData, pb.StatusCode, error) {
	file, err := srv.store.GetByHeight(ctx, id.Height)
	if errors.Is(err, store.ErrNotFound) {
		return nil, pb.StatusCode_NOT_FOUND, nil
	}
	if err != nil {
		return nil, pb.StatusCode_INTERNAL, fmt.Errorf("retrieving DAH: %w", err)
	}
	defer utils.CloseAndLog(log, "file", file)

	nd, err := eds.NamespacedData(ctx, file, id.DataNamespace)
	if err != nil {
		return nil, pb.StatusCode_INTERNAL, fmt.Errorf("getting nd: %w", err)
	}

	return nd, pb.StatusCode_OK, nil
}

func (srv *Server) respondStatus(
	ctx context.Context,
	logger *zap.SugaredLogger,
	stream network.Stream,
	status pb.StatusCode,
) error {
	srv.observeStatus(ctx, status)

	err := stream.SetWriteDeadline(time.Now().Add(srv.params.ServerWriteTimeout))
	if err != nil {
		logger.Debugw("setting write deadline", "err", err)
	}

	_, err = serde.Write(stream, &pb.GetSharesByNamespaceStatusResponse{Status: status})
	if err != nil {
		return fmt.Errorf("writing response: %w", err)
	}

	return nil
}

// sendNamespacedShares encodes shares into proto messages and sends it to client
func (srv *Server) sendNamespacedShares(data shwap.NamespacedData, stream network.Stream) error {
	for _, row := range data {
		row := &pb.NamespaceRowResponse{
			Shares: row.Shares,
			Proof: &nmt_pb.Proof{
				Start:                 int64(row.Proof.Start()),
				End:                   int64(row.Proof.End()),
				Nodes:                 row.Proof.Nodes(),
				LeafHash:              row.Proof.LeafHash(),
				IsMaxNamespaceIgnored: row.Proof.IsMaxNamespaceIDIgnored(),
			},
		}
		_, err := serde.Write(stream, row)
		if err != nil {
			return fmt.Errorf("writing nd data to stream: %w", err)
		}
	}
	return nil
}

func (srv *Server) observeStatus(ctx context.Context, status pb.StatusCode) {
	switch {
	case status == pb.StatusCode_OK:
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusSuccess)
	case status == pb.StatusCode_NOT_FOUND:
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusNotFound)
	case status == pb.StatusCode_INTERNAL:
		srv.metrics.ObserveRequests(ctx, 1, shrex.StatusInternalErr)
	}
}
