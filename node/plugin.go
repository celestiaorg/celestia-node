package node

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/node/fxutil"
)

type Plugin interface {
	Name() string
	Initialize(path string) error
	Components(cfg *Config, store Store) fxutil.Option
}

// RootPlugin strictly serves as a type that composes the Node struct. This
// provides plugins a way to force fx to load the desired plugin components
type RootPlugin struct{}

// PluginResult should be returned by at least one of the plugin's components to
// ensure that those components are added to celestia-node
type PluginResult interface{}

func collectSubOutlets(s ...PluginResult) RootPlugin {
	return RootPlugin{}
}

func collectComponents() fxutil.Option {
	return fxutil.Raw(
		fx.Provide(
			fx.Annotate(
				collectSubOutlets,
				fx.ParamTags(`group:"plugins"`),
			),
		),
	)
}
