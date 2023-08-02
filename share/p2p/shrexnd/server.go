package shrexnd

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/minio/sha256-simd"
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
	getter  share.Getter
	store   *eds.Store

	params     *Parameters
	middleware *p2p.Middleware
	metrics    *p2p.Metrics
}

// NewServer creates new Server
func NewServer(params *Parameters, host host.Host, store *eds.Store, getter share.Getter) (*Server, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-nd: server creation failed: %w", err)
	}

	srv := &Server{
		getter:     getter,
		store:      store,
		host:       host,
		params:     params,
		protocolID: p2p.ProtocolID(params.NetworkID(), protocolString),
		middleware: p2p.NewMiddleware(params.ConcurrencyLimit),
	}

	ctx, cancel := context.WithCancel(context.Background())
	srv.cancel = cancel

	handler := func(s network.Stream) {
		srv.handleNamespacedData(ctx, s)
	}
	srv.handler = srv.middleware.RateLimitHandler(handler)

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

func (srv *Server) handleNamespacedData(ctx context.Context, stream network.Stream) {
	logger := log.With("peer", stream.Conn().RemotePeer().String())
	logger.Debug("server: handling nd request")

	srv.observeRateLimitedRequests()

	err := stream.SetReadDeadline(time.Now().Add(srv.params.ServerReadTimeout))
	if err != nil {
		logger.Debugw("server: setting read deadline", "err", err)
	}

	var req pb.GetSharesByNamespaceRequest
	_, err = serde.Read(stream, &req)
	if err != nil {
		logger.Warnw("server: reading request", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}
	logger = logger.With("namespace", hex.EncodeToString(req.Namespace), "hash", share.DataHash(req.RootHash).String())
	logger.Debugw("server: new request")

	err = stream.CloseRead()
	if err != nil {
		logger.Debugw("server: closing read side of the stream", "err", err)
	}

	err = validateRequest(req)
	if err != nil {
		logger.Warnw("server: invalid request", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}

	ctx, cancel := context.WithTimeout(ctx, srv.params.HandleRequestTimeout)
	defer cancel()

	dah, err := srv.store.GetDAH(ctx, req.RootHash)
	if err != nil {
		if errors.Is(err, eds.ErrNotFound) {
			logger.Warn("server: DAH not found")
			srv.respondNotFoundError(ctx, logger, stream)
			return
		}
		logger.Errorw("server: retrieving DAH", "err", err)
		srv.respondInternalError(ctx, logger, stream)
		return
	}

	shares, err := srv.getter.GetSharesByNamespace(ctx, dah, req.Namespace)
	switch {
	case errors.Is(err, share.ErrNotFound):
		logger.Warn("server: nd not found")
		srv.respondNotFoundError(ctx, logger, stream)
		return
	case err != nil:
		logger.Errorw("server: retrieving shares", "err", err)
		srv.respondInternalError(ctx, logger, stream)
		return
	}

	resp := namespacedSharesToResponse(shares)
	srv.respond(ctx, logger, stream, resp)
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

// respondNotFoundError sends a not found response to client
func (srv *Server) respondNotFoundError(ctx context.Context,
	logger *zap.SugaredLogger, stream network.Stream) {
	resp := &pb.GetSharesByNamespaceResponse{
		Status: pb.StatusCode_NOT_FOUND,
	}
	srv.respond(ctx, logger, stream, resp)
}

// respondInternalError sends internal error response to client
func (srv *Server) respondInternalError(ctx context.Context,
	logger *zap.SugaredLogger, stream network.Stream) {
	resp := &pb.GetSharesByNamespaceResponse{
		Status: pb.StatusCode_INTERNAL,
	}
	srv.respond(ctx, logger, stream, resp)
}

// namespacedSharesToResponse encodes shares into proto and sends it to client with OK status code
func namespacedSharesToResponse(shares share.NamespacedShares) *pb.GetSharesByNamespaceResponse {
	rows := make([]*pb.Row, 0, len(shares))
	for _, row := range shares {
		row := &pb.Row{
			Shares: row.Shares,
			Proof: &nmt_pb.Proof{
				Start:                 int64(row.Proof.Start()),
				End:                   int64(row.Proof.End()),
				Nodes:                 row.Proof.Nodes(),
				LeafHash:              row.Proof.LeafHash(),
				IsMaxNamespaceIgnored: row.Proof.IsMaxNamespaceIDIgnored(),
			},
		}

		rows = append(rows, row)
	}

	status := pb.StatusCode_OK
	if len(shares) == 0 || (len(shares) == 1 && len(shares[0].Shares) == 0) {
		status = pb.StatusCode_NAMESPACE_NOT_FOUND
	}

	return &pb.GetSharesByNamespaceResponse{
		Status: status,
		Rows:   rows,
	}
}

func (srv *Server) respond(ctx context.Context,
	logger *zap.SugaredLogger, stream network.Stream, resp *pb.GetSharesByNamespaceResponse) {
	err := stream.SetWriteDeadline(time.Now().Add(srv.params.ServerWriteTimeout))
	if err != nil {
		logger.Debugw("server: setting write deadline", "err", err)
	}

	_, err = serde.Write(stream, resp)
	if err != nil {
		logger.Warnw("server: writing response", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}

	switch {
	case resp.Status == pb.StatusCode_OK:
		srv.metrics.ObserveRequests(ctx, 1, p2p.StatusSuccess)
	case resp.Status == pb.StatusCode_NOT_FOUND:
		srv.metrics.ObserveRequests(ctx, 1, p2p.StatusNotFound)
	case resp.Status == pb.StatusCode_INTERNAL:
		srv.metrics.ObserveRequests(ctx, 1, p2p.StatusInternalErr)
	}
	if err = stream.Close(); err != nil {
		logger.Debugw("server: closing stream", "err", err)
	}
}
