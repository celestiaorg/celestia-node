package das

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/das"
)

// ErrMisConfig is an error that is returned by (*Config).Validate
// when supplied with invalid values.
var ErrMisConfig = fmt.Errorf("moddas misconfiguration")

// Config contains configuration parameters for the DASer (or DASing process)
type Config das.Parameters

// TODO(@derrandz): parameters needs performance testing on real network to define optimal values
// DefaultConfig provide the optimal default configuration per node type.
// For the moment, there is only one default configuration for all node types
// but this function will provide more once #1261 is addressed.
//
// TODO(@derrandz): Address #1261
func DefaultConfig() Config {
	return Config(das.DefaultParameters())
}

// Validate performs basic validation of the config.
// Upon encountering an invalid value, Validate returns an error of type: ErrMisConfig
func (cfg *Config) Validate() error {
	err := (*das.Parameters)(cfg).Validate()

	if err != nil {
		return fmt.Errorf("%w: %s", ErrMisConfig, err)
	}

	return nil
}
