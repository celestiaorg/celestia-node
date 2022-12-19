package shrex

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/minio/sha256-simd"
	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
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
	store        *eds.Store
	host         host.Host
	writeTimeout time.Duration
	readTimeout  time.Duration
	serveTimeout time.Duration

	// TODO: will be removed after CARBlockstore is implemented to return DAH
	testDAH *share.Root
}

func StartServer(host host.Host, store *eds.Store) *Server {
	srv := &Server{
		store:        store,
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
		log.Errorw("read msg", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}

	err = stream.CloseRead()
	if err != nil {
		log.Debugf("close read side of the stream: %s", err)
	}

	err = validateRequest(req)
	if err != nil {
		log.Errorw("invalid request", "err", err)
		stream.Reset() //nolint:errcheck
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), srv.serveTimeout)
	defer cancel()

	bs, dah, err := srv.store.CARBlockstore(ctx, req.RootHash)
	if err != nil {
		log.Errorw("get header for datahash", "err", err)
		srv.respondInternal(stream)
		return
	}

	// replace with testDAH with test one if in test
	if srv.testDAH != nil {
		dah = srv.testDAH
	}

	getter := eds.NewBlockGetter(bs)
	blob, err := getBlobByNamespace(ctx, getter, dah, req.NamespaceId)
	if err != nil {
		log.Errorw("get shares", "err", err)
		srv.respondInternal(stream)
		return
	}

	resp := blobToResponse(blob)
	srv.respond(stream, resp)
}

func getBlobByNamespace(
	ctx context.Context,
	getter blockservice.BlockGetter,
	root *share.Root,
	nID namespace.ID,
) (*share.Blob, error) {
	rowRootCIDs := make([]cid.Cid, 0)
	for _, row := range root.RowsRoots {
		if !nID.Less(nmt.MinNamespace(row, nID.Size())) && nID.LessOrEqual(nmt.MaxNamespace(row, nID.Size())) {
			rowRootCIDs = append(rowRootCIDs, ipld.MustCidFromNamespacedSha256(row))
		}
	}
	if len(rowRootCIDs) == 0 {
		return nil, nil
	}

	errGroup, ctx := errgroup.WithContext(ctx)
	verifiedShares := make([]share.VerifiedShares, len(rowRootCIDs))
	for i, rootCID := range rowRootCIDs {
		// shadow loop variables, to ensure correct values are captured
		i, rootCID := i, rootCID
		errGroup.Go(func() (err error) {
			proof := new(ipld.Proof)
			shares, err := share.GetSharesByNamespace(ctx, getter, rootCID, nID, len(root.RowsRoots), proof)
			verifiedShares[i].Shares = shares
			verifiedShares[i].Proof = proof
			return
		})
	}

	if err := errGroup.Wait(); err != nil {
		return nil, err
	}
	return &share.Blob{Rows: verifiedShares}, nil
}

// validateRequest checks correctness of the request
func validateRequest(req share_p2p_v1.GetSharesByNamespaceRequest) error {
	if len(req.NamespaceId) != ipld.NamespaceSize {
		return fmt.Errorf("incorrect namespace id length: %v", len(req.NamespaceId))
	}
	if len(req.RootHash) != sha256.Size {
		return fmt.Errorf("incorrect root hash length: %v", len(req.RootHash))
	}

	return nil
}

// respondInternal sends internal error response to client
func (srv *Server) respondInternal(stream network.Stream) {
	resp := &share_p2p_v1.GetSharesByNamespaceResponse{
		Status: share_p2p_v1.StatusCode_INTERNAL,
	}
	srv.respond(stream, resp)
}

// blobToResponse encodes shares into proto and sends it to client with OK status code
func blobToResponse(blob *share.Blob) *share_p2p_v1.GetSharesByNamespaceResponse {
	rows := make([]*share_p2p_v1.Row, 0, len(blob.Rows))
	for _, row := range blob.Rows {
		// construct proof
		var proof *share_p2p_v1.Proof
		if row.Proof != nil {
			nodes := make([][]byte, 0, len(row.Proof.Nodes))
			for _, cid := range row.Proof.Nodes {
				nodes = append(nodes, cid.Bytes())
			}

			proof = &share_p2p_v1.Proof{
				Start: int64(row.Proof.Start),
				End:   int64(row.Proof.End),
				Nodes: nodes,
			}
		}

		row := &share_p2p_v1.Row{
			Shares: row.Shares,
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
