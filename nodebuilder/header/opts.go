package header

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/header"
	libhead "github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/libs/header/p2p"
	"go.uber.org/fx"
)

func WithMetrics(p2pExchange libhead.Exchange[*header.ExtendedHeader]) fx.Option {
	log.Debug("WithMetrics: replacing p2p.Exchange with p2p.ExchangeProxy")

	ex, ok := p2pExchange.(*p2p.Exchange[*header.ExtendedHeader])
	if !ok {
		err := fmt.Errorf("WithMetrics: provided exchange (p2pExchange) is not of type p2p.Exchange")
		panic(err)
	}

	return fx.Replace(p2p.NewExchangeProxy(ex))
}
