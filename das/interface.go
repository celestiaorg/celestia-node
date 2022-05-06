package das

import (
	"context"

	extheader "github.com/celestiaorg/celestia-node/service/header/extheader"
)

// HeaderGetter contains the behavior necessary for the DASer to retrieve
// headers that have been processed during header sync in order to
// perform data availability sampling over them.
type HeaderGetter interface {
	// GetByHeight returns the ExtendedHeader corresponding to the given
	// block height.
	GetByHeight(context.Context, uint64) (*extheader.ExtendedHeader, error)
}
