package shrex

import (
	"context"
	"errors"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/network"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
	share_p2p_v1 "github.com/celestiaorg/celestia-node/share/p2p/v1/pb"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

const ndProtocolID = "shrEx/nd"

var log = logging.Logger("share/server")

const (
	writeTimeout = time.Second * 10
	readTimeout  = time.Second * 10
	serveTimeout = time.Second * 10
)

type Handler struct {
	getter       Getter
	writeTimeout time.Duration
	readTimeout  time.Duration
	serveTimeout time.Duration
}

func NewAvailabilityHandler(getter Getter) *Handler {
	return &Handler{
		getter:       getter,
		writeTimeout: writeTimeout,
		readTimeout:  readTimeout,
		serveTimeout: serveTimeout,
	}
}

func (h *Handler) Handle(stream network.Stream) {
	err := stream.SetReadDeadline(time.Now().Add(h.readTimeout))
	if err != nil {
		log.Debugf("set read deadline: %s", err)
	}
	var req share_p2p_v1.GetSharesByNamespaceRequest
	_, err = serde.Read(stream, &req)
	if err != nil {
		log.Errorw("reading msg", "err", err)
		// TODO: do not respond if error from network
		h.respondInvalidArgument(stream)
		return
	}

	err = stream.CloseRead()
	if err != nil {
		log.Debugf("close read: %s", err)
	}

	err = validateRequest(req)
	if err != nil {
		log.Errorw("invalid request", "err", err)
		h.respondInvalidArgument(stream)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), h.serveTimeout)
	defer cancel()

	sharesWithProofs, err := h.getter.GetSharesWithProofsByNamespace(
		ctx, req.RootHash, req.RowRoots, int(req.MaxShares), req.NamespaceId, req.CollectProofs)
	if err != nil {
		log.Errorw("get shares", "err", err)
		h.respondInternal(stream)
		return
	}

	h.respondOK(stream, sharesWithProofs)
}

// validateRequest checks correctness of the request
func validateRequest(req share_p2p_v1.GetSharesByNamespaceRequest) error {
	if len(req.RowRoots) == 0 {
		return errors.New("no roots provided")
	}
	if len(req.NamespaceId) != ipld.NamespaceSize {
		return fmt.Errorf("incorrect namespace id length: %v", len(req.NamespaceId))
	}
	return nil
}

// respondInvalidArgument sends invalid argument response to client
func (h *Handler) respondInvalidArgument(stream network.Stream) {
	resp := &share_p2p_v1.GetSharesByNamespaceResponse{
		Status: share_p2p_v1.StatusCode_INVALID_ARGUMENT,
	}
	h.respond(stream, resp)
}

// respondInternal sends internal error response to client
func (h *Handler) respondInternal(stream network.Stream) {
	resp := &share_p2p_v1.GetSharesByNamespaceResponse{
		Status: share_p2p_v1.StatusCode_INTERNAL,
	}
	h.respond(stream, resp)
}

// respondOK encodes shares into proto and sends it to client with OK status code
func (h *Handler) respondOK(stream network.Stream, shares []share.SharesWithProof) {
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

	resp := &share_p2p_v1.GetSharesByNamespaceResponse{
		Status: share_p2p_v1.StatusCode_OK,
		Rows:   rows,
	}
	h.respond(stream, resp)
}

func (h *Handler) respond(stream network.Stream, resp *share_p2p_v1.GetSharesByNamespaceResponse) {
	err := stream.SetWriteDeadline(time.Now().Add(h.writeTimeout))
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
