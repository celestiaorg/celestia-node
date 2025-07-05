//revive:disable:var-naming
package share_v1

import (
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
)

// Config represents configuration for the share_v1 module.
// Since this module reuses the core infrastructure from the original share module,
// it primarily references the existing share configuration.
type Config struct {
	// BaseShareConfig references the original share module configuration
	// to ensure consistency across both modules
	BaseShareConfig *share.Config
}

// DefaultConfig returns the default configuration for share_v1 module
func DefaultConfig(tp node.Type) *Config {
	baseConfig := share.DefaultConfig(tp)
	return &Config{
		BaseShareConfig: &baseConfig,
	}
}

// Validate validates the share_v1 configuration
func (cfg *Config) Validate(tp node.Type) error {
	if cfg.BaseShareConfig == nil {
		baseConfig := share.DefaultConfig(tp)
		cfg.BaseShareConfig = &baseConfig
	}
	return cfg.BaseShareConfig.Validate(tp)
}
