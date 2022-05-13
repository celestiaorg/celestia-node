package state

import (
	"github.com/celestiaorg/celestia-node/service/state"
)

func NewService(accessor state.Accessor) *state.Service {
	return state.NewService(accessor)
}
