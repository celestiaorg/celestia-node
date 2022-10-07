package rpc

import (
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

var log = logging.Logger("rpc")

type Handler struct {
	state  state.Module
	share  share.Module
	header header.Module
	das    *das.DASer
}

func NewHandler(
	state state.Module,
	share share.Module,
	header header.Module,
	das *das.DASer,
) *Handler {
	return &Handler{
		state:  state,
		share:  share,
		header: header,
		das:    das,
	}
}

func (h *Handler) RegisterEndpoints(rpc *Server) {
	rpc.rpc.Register("State", h.state)
	rpc.rpc.Register("Share", h.share)
	rpc.rpc.Register("Header", h.header)
	rpc.rpc.Register("DAS", h.das)
}
