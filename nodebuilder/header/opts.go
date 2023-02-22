package header

import (
	"fmt"

	header "github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	p2p "github.com/celestiaorg/celestia-node/libs/header/p2p"
)

// WithMetrics provides sets `MetricsEnabled` to true on ClientParameters for the header exchange
func WithMetrics(ex libhead.Exchange[*header.ExtendedHeader]) error {
	switch any(ex).(type) {
	case *p2p.Exchange[*header.ExtendedHeader]:
		exchange, ok := (ex).(*p2p.Exchange[*header.ExtendedHeader])
		if !ok {
			return fmt.Errorf("header.WithMetrics: type is to *p2p.Exchange but cast failed")
		}
		return exchange.RegisterMetrics()
	default:
		return nil
	}
}
