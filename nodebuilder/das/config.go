package das

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/das"
)

// Config contains configuration parameters for the DASer (or DASing process)
type Config das.Parameters

// TODO(@derrandz): parameters needs performance testing on real network to define optimal values
func DefaultConfig() Config {
	return Config(das.DefaultParameters())
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	err := (*das.Parameters)(cfg).Validate()

	if err != nil {
		return fmt.Errorf("moddas misconfiguration: %w", err)
	}

	return nil
}
