package rpc

import (
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/share"
	"github.com/celestiaorg/celestia-node/service/state"
)

var log = logging.Logger("rpc")

type Handler struct {
	state  *state.Service
	share  share.Service
	header *header.Service
}

func NewHandler(state *state.Service, share share.Service, header *header.Service) *Handler {
	return &Handler{
		state:  state,
		share:  share,
		header: header,
	}
}
