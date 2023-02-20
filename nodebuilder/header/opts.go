package header

import (
	header "github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
)

// WithMetrics provides sets `MetricsEnabled` to true on ClientParameters for the header exchange
func WithMetrics(ex libhead.Exchange[*header.ExtendedHeader]) error {
	return ex.RegisterMetrics()
}
