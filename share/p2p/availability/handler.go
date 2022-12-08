package availability

import (
	"fmt"
	"github.com/celestiaorg/celestia-node/header"
	header_svc "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	share2 "github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/p2p"
	share_p2p_v1 "github.com/celestiaorg/celestia-node/share/p2p/v1/pb"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/protocol"
)

const protocolID = "shrEx/nd"

var log = logging.Logger("share/server")

type Handler struct {
	share  share.Module
	header header_svc.Module
}

func NewAvailabilityHandler(share share.Module, header header_svc.Module) *Handler {
	return &Handler{
		share:  share,
		header: header,
	}
}

func (h *Handler) ProtocolID() protocol.ID {
	return protocolID
}

func (h *Handler) Handle(s *p2p.Session) error {
	var req share_p2p_v1.GetSharesByNamespaceRequest
	err := s.Read(&req)
	if err != nil {
		// TODO: handle unmarshall errors as incorrect input
		return fmt.Errorf("reading msg: %w", err)
	}

	var header *header.ExtendedHeader
	switch req.Height {
	case 0:
		header, err = h.header.Head(s.Ctx)
	default:
		header, err = h.header.GetByHeight(s.Ctx, uint64(req.Height))
	}
	if err != nil {
		h.respondInternal(s)
		return fmt.Errorf("getting header: %w", err)
	}

	sharesWithProofs, err := h.share.GetSharesWithProofsByNamespace(s.Ctx, header.DAH, req.NamespaceId)
	if err != nil {
		h.respondInternal(s)
		return fmt.Errorf("getting shares: %w", err)
	}

	resp := prepareResponse(sharesWithProofs)
	err = s.Write(resp)
	if err != nil {
		return fmt.Errorf("writing msg: %w", err)
	}
	return nil
}

func (h *Handler) respondInternal(s *p2p.Session) {
	resp := &share_p2p_v1.GetSharesByNamespaceResponse{
		Status: share_p2p_v1.StatusCode_INTERNAL,
	}
	err := s.Write(resp)
	if err != nil {
		log.Error("unable to response internal: %w", err)
	}
}

func prepareResponse(shares []share2.SharesWithProof) *share_p2p_v1.GetSharesByNamespaceResponse {
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

	return &share_p2p_v1.GetSharesByNamespaceResponse{
		Status: share_p2p_v1.StatusCode_OK,
		Rows:   rows,
	}
}
