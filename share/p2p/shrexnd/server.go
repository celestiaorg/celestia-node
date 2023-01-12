package shrexnd

import (
	"context"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/minio/sha256-simd"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/getters"
	"github.com/celestiaorg/celestia-node/share/ipld"
	pb "github.com/celestiaorg/celestia-node/share/p2p/shrexnd/pb"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

const ndProtocolID = "/shrex/nd/0.0.1"

var log = logging.Logger("shrex/nd")

const (
	writeTimeout = time.Second * 10
	readTimeout  = time.Second * 10
	serveTimeout = time.Second * 10
)

// Server implements server side of shrex/nd protocol to obtain namespaced shares data from remote
// peers.
type Server struct {
	getter       share.Getter
	store        *eds.Store
	host         host.Host
	writeTimeout time.Duration
	readTimeout  time.Duration
	serveTimeout time.Duration

	cancel context.CancelFunc
}

// NewServer creates new Server
func NewServer(host host.Host, store *eds.Store, getter *getters.IPLDGetter) *Server {
	srv := &Server{
		getter:       getter,
		store:        store,
		host:         host,
		writeTimeout: writeTimeout,
		readTimeout:  readTimeout,
		serveTimeout: serveTimeout,
	}

	return srv
}

// Start starts the server
func (srv *Server) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	srv.cancel = cancel

	srv.host.SetStreamHandler(ndProtocolID, func(s network.Stream) {
		srv.handleNamespacedData(ctx, s)
	})
}

// Stop stops the server
func (srv *Server) Stop() {
	srv.cancel()
	srv.host.RemoveStreamHandler(ndProtocolID)
}

func (srv *Server) handleNamespacedData(ctx context.Context, stream network.Stream) {
	err := stream.SetReadDeadline(time.Now().Add(srv.readTimeout))
	if err != nil {
		log.Debugf("server: seting read deadline: %s", err)
	}

	var req pb.GetSharesByNamespaceRequest
	_, err = serde.Read(stream, &req)
	if err != nil {
		log.Errorw("server: reading request", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}

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

	ctx, cancel := context.WithTimeout(ctx, srv.serveTimeout)
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
	err := stream.SetWriteDeadline(time.Now().Add(srv.writeTimeout))
	if err != nil {
		log.Debugf("server: seting write deadline: %s", err)
	}

	_, err = serde.Write(stream, resp)
	if err != nil {
		log.Errorf("server: writing response: %s", err.Error())
		stream.Reset() //nolint:errcheck
		return
	}
	stream.Close()
}
