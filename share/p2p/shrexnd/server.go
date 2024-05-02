package shrexnd

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/zap"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
	nmt_pb "github.com/celestiaorg/nmt/pb"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/p2p"
	pb "github.com/celestiaorg/celestia-node/share/p2p/shrexnd/pb"
)

// Server implements server side of shrex/nd protocol to serve namespaced share to remote
// peers.
type Server struct {
	cancel context.CancelFunc

	host       host.Host
	protocolID protocol.ID

	handler network.StreamHandler
	store   *eds.Store

	params     *Parameters
	middleware *p2p.Middleware
	metrics    *p2p.Metrics
}

// NewServer creates new Server
func NewServer(params *Parameters, host host.Host, store *eds.Store) (*Server, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-nd: server creation failed: %w", err)
	}

	srv := &Server{
		store:      store,
		host:       host,
		params:     params,
		protocolID: p2p.ProtocolID(params.NetworkID(), protocolString),
		middleware: p2p.NewMiddleware(params.ConcurrencyLimit),
	}

	ctx, cancel := context.WithCancel(context.Background())
	srv.cancel = cancel

	handler := srv.streamHandler(ctx)
	withRateLimit := srv.middleware.RateLimitHandler(handler)
	withRecovery := p2p.RecoveryMiddleware(withRateLimit)
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
		srv.metrics.ObserveRequests(context.Background(), numRateLimited, p2p.StatusRateLimited)
	}
}

func (srv *Server) handleNamespacedData(ctx context.Context, stream network.Stream) error {
	logger := log.With("source", "server", "peer", stream.Conn().RemotePeer().String())
	logger.Debug("handling nd request")

	srv.observeRateLimitedRequests()
	req, err := srv.readRequest(logger, stream)
	if err != nil {
		logger.Warnw("read request", "err", err)
		srv.metrics.ObserveRequests(ctx, 1, p2p.StatusBadRequest)
		return err
	}

	logger = logger.With("namespace", share.Namespace(req.Namespace).String(),
		"hash", share.DataHash(req.RootHash).String())

	ctx, cancel := context.WithTimeout(ctx, srv.params.HandleRequestTimeout)
	defer cancel()

	shares, status, err := srv.getNamespaceData(ctx, req.RootHash, req.Namespace)
	if err != nil {
		// server should respond with status regardless if there was an error getting data
		sendErr := srv.respondStatus(ctx, logger, stream, status)
		if sendErr != nil {
			logger.Errorw("sending response", "err", sendErr)
			srv.metrics.ObserveRequests(ctx, 1, p2p.StatusSendRespErr)
		}
		logger.Errorw("handling request", "err", err)
		return errors.Join(err, sendErr)
	}

	err = srv.respondStatus(ctx, logger, stream, status)
	if err != nil {
		logger.Errorw("sending response", "err", err)
		srv.metrics.ObserveRequests(ctx, 1, p2p.StatusSendRespErr)
		return err
	}

	err = srv.sendNamespacedShares(shares, stream)
	if err != nil {
		logger.Errorw("send nd data", "err", err)
		srv.metrics.ObserveRequests(ctx, 1, p2p.StatusSendRespErr)
		return err
	}
	return nil
}

func (srv *Server) readRequest(
	logger *zap.SugaredLogger,
	stream network.Stream,
) (*pb.GetSharesByNamespaceRequest, error) {
	err := stream.SetReadDeadline(time.Now().Add(srv.params.ServerReadTimeout))
	if err != nil {
		logger.Debugw("setting read deadline", "err", err)
	}

	var req pb.GetSharesByNamespaceRequest
	_, err = serde.Read(stream, &req)
	if err != nil {
		return nil, fmt.Errorf("reading request: %w", err)
	}

	logger.Debugw("new request")
	err = stream.CloseRead()
	if err != nil {
		logger.Debugw("closing read side of the stream", "err", err)
	}

	err = validateRequest(req)
	if err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	return &req, nil
}

func (srv *Server) getNamespaceData(ctx context.Context,
	hash share.DataHash, namespace share.Namespace,
) (share.NamespacedShares, pb.StatusCode, error) {
	dah, err := srv.store.GetDAH(ctx, hash)
	if err != nil {
		if errors.Is(err, eds.ErrNotFound) {
			return nil, pb.StatusCode_NOT_FOUND, nil
		}
		return nil, pb.StatusCode_INTERNAL, fmt.Errorf("retrieving DAH: %w", err)
	}

	shares, err := eds.RetrieveNamespaceFromStore(ctx, srv.store, dah, namespace)
	if err != nil {
		return nil, pb.StatusCode_INTERNAL, fmt.Errorf("retrieving shares: %w", err)
	}

	return shares, pb.StatusCode_OK, nil
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
func (srv *Server) sendNamespacedShares(shares share.NamespacedShares, stream network.Stream) error {
	for _, row := range shares {
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
		srv.metrics.ObserveRequests(ctx, 1, p2p.StatusSuccess)
	case status == pb.StatusCode_NOT_FOUND:
		srv.metrics.ObserveRequests(ctx, 1, p2p.StatusNotFound)
	case status == pb.StatusCode_INTERNAL:
		srv.metrics.ObserveRequests(ctx, 1, p2p.StatusInternalErr)
	}
}

// validateRequest checks correctness of the request
func validateRequest(req pb.GetSharesByNamespaceRequest) error {
	if err := share.Namespace(req.Namespace).ValidateForData(); err != nil {
		return err
	}
	if len(req.RootHash) != sha256.Size {
		return fmt.Errorf("incorrect root hash length: %v", len(req.RootHash))
	}
	return nil
}
