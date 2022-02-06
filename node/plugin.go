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

// RootPluginOutlet strictly serves as a type to be returned by a plugin's
// fxutil.Option. This provides the plugin a way to force fx to call the
// plugin's services/components
type RootPluginOutlet struct{}

type PluginOutlet interface{}

func CollectSubOutlets(s ...PluginOutlet) RootPluginOutlet {
	return RootPluginOutlet{}
}

func collectComponents(cfg *Config, store Store) fxutil.Option {
	return fxutil.Raw(
		fx.Provide(
			fx.Annotate(
				CollectSubOutlets,
				fx.ParamTags(`group:"plugins"`),
			),
		),
	)
}
