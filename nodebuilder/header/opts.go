package header

import (
	"github.com/celestiaorg/celestia-node/libs/header/p2p"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"go.uber.org/fx"
)

// WithMetrics provides sets `MetricsEnabled` to true on ClientParameters for the header exchange
func WithMetrics() fx.Option {
	return fx.Provide(
		func(cfg Config, network modp2p.Network) []p2p.Option[p2p.ClientParameters] {
			return []p2p.Option[p2p.ClientParameters]{
				p2p.WithMetrics(),
			}
		},
	)
}
