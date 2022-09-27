package nodebuilder

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/daser"

	headermodule "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/params"
)

// WithNetwork specifies the Network to which the Node should connect to.
// WARNING: Use this option with caution and never run the Node with different networks over the same persisted Store.
func WithNetwork(net params.Network) fx.Option {
	return fx.Replace(net)
}

// WithBootstrappers sets custom bootstrap peers.
func WithBootstrappers(peers params.Bootstrappers) fx.Option {
	return fx.Replace(peers)
}

// WithMetrics enables metrics exporting for the node.
func WithMetrics(enable bool) fx.Option {
	return fx.Options(
		// TODO: Add metrics to more modules and enable here.
		headermodule.WithMetrics(enable),
		daser.WithMetrics(enable),
	)
}
