package pruner

import (
	"context"

	"github.com/celestiaorg/celestia-node/header"
)

// Factory contains methods necessary to prune data
// from the node's datastore.
type Factory interface {
	AvailabilityWindow
	Prune(context.Context, ...*header.ExtendedHeader) error
}
