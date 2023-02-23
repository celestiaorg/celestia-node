package header

import (
	header "github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	p2p "github.com/celestiaorg/celestia-node/libs/header/p2p"
)

// WithMetrics provides sets `MetricsEnabled` to true on ClientParameters for the header exchange
func WithMetrics(ex libhead.Exchange[*header.ExtendedHeader]) error {
	switch p2pex := any(ex).(type) {
	case *p2p.Exchange[*header.ExtendedHeader]:
		return p2pex.RegisterMetrics()
	default:
		return nil
	}
}
