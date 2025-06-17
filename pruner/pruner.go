package pruner

import (
	"context"
)

// Pruner contains methods necessary to prune data
// from the node's datastore.
type Pruner interface {
	Prune(ctx context.Context, height uint64) error
}
