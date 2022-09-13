package nodebuilder

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/daser"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/params"
	"github.com/celestiaorg/celestia-node/state"
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
func WithMetrics(enable bool, nodeType node.Type) fx.Option {
	if !enable {
		return fx.Options()
	}
	var opts fx.Option
	switch nodeType {
	case node.Full:
		opts = fx.Options(
			fx.Invoke(header.MonitorHead),
			fx.Invoke(state.MonitorPFDs),
			fx.Invoke(daser.MonitorDASer),
			// add more monitoring here
		)
	case node.Bridge:
		opts = fx.Options(
			fx.Invoke(header.MonitorHead),
			fx.Invoke(state.MonitorPFDs),
			fx.Invoke(core.MonitorBroadcasting),
			// add more monitoring here
		)
	case node.Light:
		opts = fx.Options(
			fx.Invoke(header.MonitorHead),
			fx.Invoke(state.MonitorPFDs),
			fx.Invoke(daser.MonitorDASer),
			// add more monitoring here
		)
	default:
		panic("invalid node type")
	}
	return opts
}
