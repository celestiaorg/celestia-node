package node

import "github.com/celestiaorg/celestia-node/node/fxutil"

type Plugin interface {
	Name() string
	Initialize(path string) error
	Components(cfg *Config, store Store) fxutil.Option
}

// PluginOutlet strictly serves as a type to be returned by a plugin's
// fxutil.Option. This provides the plugin a way to force fx to call the
// plugin's services/components
type PluginOutlet struct{}

type nullPlugin struct{}

func (nullPlugin) Name() string                 { return "nullPlugin" }
func (nullPlugin) Initialize(path string) error { return nil }
func (nullPlugin) Components(cfg *Config, store Store) fxutil.Option {
	return fxutil.Options(
		fxutil.Supply(PluginOutlet{}),
	)
}
