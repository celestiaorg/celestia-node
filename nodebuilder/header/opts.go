package header

import (
	header "github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	p2p "github.com/celestiaorg/celestia-node/libs/header/p2p"
)

// WithMetrics provides sets `MetricsEnabled` to true on ClientParameters for the header exchange
func WithMetrics(ex libhead.Exchange[*header.ExtendedHeader]) error {
	exchange, ok := (ex).(*p2p.Exchange[*header.ExtendedHeader])
	if !ok {
		// not all implementations of libhead.Exchange[*header.ExtendedHeader]
		// are p2p.Exchange[*header.ExtendedHeader
		// thus we need to avoid panicking here for when
		// ex is of another base type
		return nil
	}
	return exchange.RegisterMetrics()
}
