package header

import (
	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/p2p"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
)

// WithMetrics provides sets `MetricsEnabled` to true on ClientParameters for the header exchange
func WithMetrics(
	store libhead.Store[*header.ExtendedHeader],
	ex libhead.Exchange[*header.ExtendedHeader],
	sync *sync.Syncer[*header.ExtendedHeader],
) error {
	if p2pex, ok := ex.(*p2p.Exchange[*header.ExtendedHeader]); ok {
		if err := p2pex.InitMetrics(); err != nil {
			return err
		}
	}

	if err := sync.InitMetrics(); err != nil {
		return err
	}

	return libhead.WithMetrics[*header.ExtendedHeader](store)
}
