package shrexnd

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/minio/sha256-simd"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/ipld"
	pb "github.com/celestiaorg/celestia-node/share/p2p/shrexnd/pb"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

// Server implements server side of shrex/nd protocol to serve namespaced share to remote
// peers.
type Server struct {
	params     *Parameters
	protocolID protocol.ID

	getter share.Getter
	store  *eds.Store
	host   host.Host

	cancel context.CancelFunc
}

// NewServer creates new Server
func NewServer(host host.Host, store *eds.Store, getter share.Getter, opts ...Option) (*Server, error) {
	params := DefaultParameters()
	for _, opt := range opts {
		opt(params)
	}

	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("shrex-nd: server creation failed: %w", err)
	}

	srv := &Server{
		getter:     getter,
		store:      store,
		host:       host,
		params:     params,
		protocolID: protocolID(params.protocolSuffix),
	}

	return srv, nil
}

// Start starts the server
func (srv *Server) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	srv.cancel = cancel

	srv.host.SetStreamHandler(srv.protocolID, func(s network.Stream) {
		srv.handleNamespacedData(ctx, s)
	})
}

// Stop stops the server
func (srv *Server) Stop() {
	srv.cancel()
	srv.host.RemoveStreamHandler(srv.protocolID)
}

func (srv *Server) handleNamespacedData(ctx context.Context, stream network.Stream) {
	err := stream.SetReadDeadline(time.Now().Add(srv.params.readTimeout))
	if err != nil {
		log.Debugf("server: setting read deadline: %s", err)
	}

	var req pb.GetSharesByNamespaceRequest
	_, err = serde.Read(stream, &req)
	if err != nil {
		log.Errorw("server: reading request", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}
	log.Debugw("server: new request", "namespaceId", string(req.NamespaceId), "roothash", string(req.RootHash))

	err = stream.CloseRead()
	if err != nil {
		log.Debugf("server: closing read side of the stream: %s", err)
	}

	err = validateRequest(req)
	if err != nil {
		log.Errorw("server: invalid request", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}

	ctx, cancel := context.WithTimeout(ctx, srv.params.serveTimeout)
	defer cancel()

	dah, err := srv.store.GetDAH(ctx, req.RootHash)
	if err != nil {
		log.Errorw("server: retrieving DAH for datahash", "err", err)
		srv.respondInternalError(stream)
		return
	}

	shares, err := srv.getter.GetSharesByNamespace(ctx, dah, req.NamespaceId)
	if err != nil {
		log.Errorw("server: retrieving shares", "err", err)
		srv.respondInternalError(stream)
		return
	}

	resp := namespacedSharesToResponse(shares)
	srv.respond(stream, resp)
	log.Debugw("server: handled request", "namespaceId", string(req.NamespaceId), "roothash", string(req.RootHash))
}

// validateRequest checks correctness of the request
func validateRequest(req pb.GetSharesByNamespaceRequest) error {
	if len(req.NamespaceId) != ipld.NamespaceSize {
		return fmt.Errorf("incorrect namespace id length: %v", len(req.NamespaceId))
	}
	if len(req.RootHash) != sha256.Size {
		return fmt.Errorf("incorrect root hash length: %v", len(req.RootHash))
	}

	return nil
}

// respondInternalError sends internal error response to client
func (srv *Server) respondInternalError(stream network.Stream) {
	resp := &pb.GetSharesByNamespaceResponse{
		Status: pb.StatusCode_INTERNAL,
	}
	srv.respond(stream, resp)
}

// namespacedSharesToResponse encodes shares into proto and sends it to client with OK status code
func namespacedSharesToResponse(shares share.NamespacedShares) *pb.GetSharesByNamespaceResponse {
	rows := make([]*pb.Row, 0, len(shares))
	for _, row := range shares {
		// construct proof
		nodes := make([][]byte, 0, len(row.Proof.Nodes))
		for _, cid := range row.Proof.Nodes {
			nodes = append(nodes, cid.Bytes())
		}

		proof := &pb.Proof{
			Start: int64(row.Proof.Start),
			End:   int64(row.Proof.End),
			Nodes: nodes,
		}

		row := &pb.Row{
			Shares: row.Shares,
			Proof:  proof,
		}

		rows = append(rows, row)
	}

	return &pb.GetSharesByNamespaceResponse{
		Status: pb.StatusCode_OK,
		Rows:   rows,
	}
}

func (srv *Server) respond(stream network.Stream, resp *pb.GetSharesByNamespaceResponse) {
	err := stream.SetWriteDeadline(time.Now().Add(srv.params.writeTimeout))
	if err != nil {
		log.Debugf("server: seting write deadline: %s", err)
	}

	_, err = serde.Write(stream, resp)
	if err != nil {
		log.Errorf("server: writing response: %s", err.Error())
		stream.Reset() //nolint:errcheck
		return
	}

	if err = stream.Close(); err != nil {
		log.Errorf("server: closing stream: %s", err.Error())
	}
}
