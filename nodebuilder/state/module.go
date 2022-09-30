package state

import (
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

var log = logging.Logger("state-module")

// ConstructModule provides all components necessary to construct the
// state service.
func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Provide(Keyring),
		fx.Provide(CoreAccessor),
	)

	switch tp {
	case node.Light, node.Full, node.Bridge:
		return fx.Module(
			"state",
			baseComponents,
		)
	default:
		panic("invalid node type")
	}
}
