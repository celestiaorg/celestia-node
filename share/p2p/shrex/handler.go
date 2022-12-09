package shrex

import (
	"context"
	"fmt"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
	header_svc "github.com/celestiaorg/celestia-node/nodebuilder/header"
	share_svc "github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/p2p"
	share_p2p_v1 "github.com/celestiaorg/celestia-node/share/p2p/v1/pb"
)

const ndProtocolID = "shrEx/nd"

var log = logging.Logger("share/server")

type Handler struct {
	share  share_svc.Module
	header header_svc.Module
}

func NewAvailabilityHandler(share share_svc.Module, header header_svc.Module) *Handler {
	return &Handler{
		share:  share,
		header: header,
	}
}

func (h *Handler) Handle(ctx context.Context, s *p2p.Session) error {
	var req share_p2p_v1.GetSharesByNamespaceRequest
	err := s.Read(&req)
	if err != nil {
		// TODO: do not try to respond if stream is already down
		h.respondInvalidArgument(s)
		return fmt.Errorf("reading msg: %w", err)
	}

	err = validateRequest(req)
	if err != nil {
		h.respondInvalidArgument(s)
		return fmt.Errorf("invalid request: %w", err)
	}

	var header *header.ExtendedHeader
	switch req.Height {
	case 0:
		header, err = h.header.Head(ctx)
	default:
		header, err = h.header.GetByHeight(ctx, uint64(req.Height))
	}
	if err != nil {
		h.respondInternal(s)
		return fmt.Errorf("getting header: %w", err)
	}

	sharesWithProofs, err := h.share.GetSharesWithProofsByNamespace(ctx, header.DAH, req.NamespaceId)
	if err != nil {
		h.respondInternal(s)
		return fmt.Errorf("getting shares: %w", err)
	}

	return respondOK(s, sharesWithProofs)
}

// validateRequest checks correctness of the request
func validateRequest(req share_p2p_v1.GetSharesByNamespaceRequest) error {
	if req.Height < 0 {
		return fmt.Errorf("height could not be less than zero: %v", req.Height)
	}
	if len(req.NamespaceId) != ipld.NamespaceSize {
		return fmt.Errorf("incorrect namespace id length: %v", len(req.NamespaceId))
	}
	return nil
}

// respondInvalidArgument sends invalid argument response to client
func (h *Handler) respondInvalidArgument(s *p2p.Session) {
	resp := &share_p2p_v1.GetSharesByNamespaceResponse{
		Status: share_p2p_v1.StatusCode_INVALID_ARGUMENT,
	}
	err := s.Write(resp)
	if err != nil {
		log.Errorf("responding invalid arg: %s", err.Error())
	}
}

// respondInternal sends internal error response to client
func (h *Handler) respondInternal(s *p2p.Session) {
	resp := &share_p2p_v1.GetSharesByNamespaceResponse{
		Status: share_p2p_v1.StatusCode_INTERNAL,
	}
	err := s.Write(resp)
	if err != nil {
		log.Errorf("responding internal: %s", err.Error())
	}
}

// respondOK encodes shares into proto and sends it to client with OK status code
func respondOK(s *p2p.Session, shares []share.SharesWithProof) error {
	rows := make([]*share_p2p_v1.Row, 0, len(shares))
	for _, sh := range shares {
		nodes := make([][]byte, 0, len(sh.Proof.Nodes))
		for _, cid := range sh.Proof.Nodes {
			nodes = append(nodes, cid.Bytes())
		}

		proof := &share_p2p_v1.Proof{
			Start: int64(sh.Proof.Start),
			End:   int64(sh.Proof.End),
			Nodes: nodes,
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

	err := s.Write(resp)
	if err != nil {
		log.Errorf("writing ok response: %s", err.Error())
	}
	return nil
}
