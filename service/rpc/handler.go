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
	state  state.Service
	share  share.Service
	header header.Service
	das    *das.DASer
}

func NewHandler(
	state state.Service,
	share share.Service,
	header header.Service,
	das *das.DASer,
) *Handler {
	return &Handler{
		state:  state,
		share:  share,
		header: header,
		das:    das,
	}
}
