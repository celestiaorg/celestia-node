package pruner

import (
	"context"

	"github.com/celestiaorg/celestia-node/header"
)

// Pruner contains methods necessary to prune data
// from the node's datastore.
type Pruner interface {
	Prune(context.Context, *header.ExtendedHeader) error
	// Kind returns the kind of the pruner implementation.
	Kind() string
}
