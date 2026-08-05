package das

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// Config contains configuration parameters for the DASer (or DASing process)
type Config struct {
	das.Parameters
	// Enabled controls whether the DASer is started at all. If false, the DASer is disabled and not invoked.
	Enabled bool `json:"enabled"`
}

// TODO(@derrandz): parameters needs performance testing on real network to define optimal values
// DefaultConfig provide the optimal default configuration per node type.
// For the moment, there is only one default configuration for all node types
// but this function will provide more once #1261 is addressed.
//
// TODO(@derrandz): Address #1261
func DefaultConfig(node.Type) Config {
	return Config{
		Parameters: das.DefaultParameters(),
		Enabled:    true, // Enabled by default
	}
}

// Validate performs basic validation of the config.
// Upon encountering an invalid value, Validate returns an error of type: ErrMisConfig
func (cfg *Config) Validate() error {
	err := cfg.Parameters.Validate()
	if err != nil {
		return fmt.Errorf("moddas misconfiguration: %w", err)
	}

	return nil
}
