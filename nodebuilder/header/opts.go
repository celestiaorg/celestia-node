package header

import (
	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/libs/header/sync"
)

// WithMetrics provides sets `MetricsEnabled` to true on ClientParameters for the header exchange
func WithMetrics(store libhead.Store[*header.ExtendedHeader], ex libhead.Exchange[*header.ExtendedHeader], sync *sync.Syncer[*header.ExtendedHeader]) error {
	return libhead.WithMetrics[*header.ExtendedHeader](store, sync, ex)
}
