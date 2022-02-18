package das

import (
	"context"

	"github.com/celestiaorg/celestia-node/service/header"
)

// HeaderGetter contains the behaviour necessary for the DASer
// to retrieve headers that have become newly available during the
// syncing process in order to perform data availability sampling
// over headers from the past.
type HeaderGetter interface {
	// Head returns the ExtendedHeader of the chain head.
	Head(context.Context) (*header.ExtendedHeader, error)
	// GetByHeight returns the ExtendedHeader corresponding to the given
	// block height.
	GetByHeight(context.Context, uint64) (*header.ExtendedHeader, error)
}
