package shrex

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
	share_p2p_v1 "github.com/celestiaorg/celestia-node/share/p2p/v1/pb"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

const ndProtocolID = "/shrex/nd/0.0.1"

var log = logging.Logger("shrex/nd")

const (
	writeTimeout = time.Second * 10
	readTimeout  = time.Second * 10
	serveTimeout = time.Second * 10
)

type Server struct {
	getter       Getter
	host         host.Host
	writeTimeout time.Duration
	readTimeout  time.Duration
	serveTimeout time.Duration
}

func StartServer(host host.Host, getter Getter) *Server {
	srv := &Server{
		getter:       getter,
		host:         host,
		writeTimeout: writeTimeout,
		readTimeout:  readTimeout,
		serveTimeout: serveTimeout,
	}
	host.SetStreamHandler(ndProtocolID, srv.handleNamespacedData)
	return srv
}

// Stop stops the server
func (srv *Server) Stop() {
	srv.host.RemoveStreamHandler(ndProtocolID)
}

func (srv *Server) handleNamespacedData(stream network.Stream) {
	err := stream.SetReadDeadline(time.Now().Add(srv.readTimeout))
	if err != nil {
		log.Debugf("set read deadline: %s", err)
	}
	var req share_p2p_v1.GetSharesByNamespaceRequest
	_, err = serde.Read(stream, &req)
	if err != nil {
		log.Errorw("reading msg", "err", err)
		// TODO: do not respond if error is from network
		srv.respondInvalidArgument(stream)
		return
	}

	err = stream.CloseRead()
	if err != nil {
		log.Debugf("close read: %s", err)
	}

	err = validateRequest(req)
	if err != nil {
		log.Errorw("invalid request", "err", err)
		srv.respondInvalidArgument(stream)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), srv.serveTimeout)
	defer cancel()

	// select rows that contain shares for given namespaceID
	nID := namespace.ID(req.NamespaceId)
	selectedRoots := make([][]byte, 0)
	for _, row := range req.RowRoots {
		if !nID.Less(nmt.MinNamespace(row, nID.Size())) && nID.LessOrEqual(nmt.MaxNamespace(row, nID.Size())) {
			selectedRoots = append(selectedRoots, row)
		}
	}
	if len(selectedRoots) == 0 {
		srv.respond(stream, sharesWithProofsToResponse(nil))
	}

	sharesWithProofs, err := srv.getter.GetSharesWithProofsByNamespace(
		ctx, req.RootHash, selectedRoots, len(selectedRoots), req.NamespaceId, req.CollectProofs)
	if err != nil {
		log.Errorw("get shares", "err", err)
		srv.respondInternal(stream)
		return
	}

	resp := sharesWithProofsToResponse(sharesWithProofs)
	srv.respond(stream, resp)
}

// validateRequest checks correctness of the request
func validateRequest(req share_p2p_v1.GetSharesByNamespaceRequest) error {
	if len(req.RowRoots) == 0 {
		return errors.New("no roots provided")
	}
	if len(req.NamespaceId) != ipld.NamespaceSize {
		return fmt.Errorf("incorrect namespace id length: %v", len(req.NamespaceId))
	}
	if len(req.RootHash) != sha256.Size {
		return fmt.Errorf("incorrect root hash length: %v", len(req.RootHash))
	}

	for _, row := range req.RowRoots {
		if len(row) != ipld.NmtHashSize {
			return fmt.Errorf("incorrect row root length: %v", len(row))
		}
	}
	return nil
}

// respondInvalidArgument sends invalid argument response to client
func (srv *Server) respondInvalidArgument(stream network.Stream) {
	resp := &share_p2p_v1.GetSharesByNamespaceResponse{
		Status: share_p2p_v1.StatusCode_INVALID_ARGUMENT,
	}
	srv.respond(stream, resp)
}

// respondInternal sends internal error response to client
func (srv *Server) respondInternal(stream network.Stream) {
	resp := &share_p2p_v1.GetSharesByNamespaceResponse{
		Status: share_p2p_v1.StatusCode_INTERNAL,
	}
	srv.respond(stream, resp)
}

// sharesWithProofsToResponse encodes shares into proto and sends it to client with OK status code
func sharesWithProofsToResponse(
	shares []share.SharesWithProof,
) *share_p2p_v1.GetSharesByNamespaceResponse {
	rows := make([]*share_p2p_v1.Row, 0, len(shares))
	for _, sh := range shares {
		// construct proof
		var proof *share_p2p_v1.Proof
		if sh.Proof != nil {
			nodes := make([][]byte, 0, len(sh.Proof.Nodes))
			for _, cid := range sh.Proof.Nodes {
				nodes = append(nodes, cid.Bytes())
			}

			proof = &share_p2p_v1.Proof{
				Start: int64(sh.Proof.Start),
				End:   int64(sh.Proof.End),
				Nodes: nodes,
			}
		}

		row := &share_p2p_v1.Row{
			Shares: sh.Shares,
			Proof:  proof,
		}

		rows = append(rows, row)
	}

	return &share_p2p_v1.GetSharesByNamespaceResponse{
		Status: share_p2p_v1.StatusCode_OK,
		Rows:   rows,
	}
}

func (srv *Server) respond(stream network.Stream, resp *share_p2p_v1.GetSharesByNamespaceResponse) {
	err := stream.SetWriteDeadline(time.Now().Add(srv.writeTimeout))
	if err != nil {
		log.Debugf("set write deadline: %s", err)
	}

	_, err = serde.Write(stream, resp)
	if err != nil {
		log.Errorf("responding: %s", err.Error())
		stream.Reset() //nolint:errcheck
		return
	}
	stream.Close()
}
