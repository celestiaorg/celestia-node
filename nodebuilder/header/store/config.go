package store

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/header/store"
)

// Config contains configuration parameters for the store
type Config store.Parameters

func DefaultConfig() Config {
	return Config(*store.DefaultParameters())
}

// Validate performs basic validation of the config.
// Upon encountering an invalid value, Validate returns an error of type: ErrMisConfig
func (cfg *Config) Validate() error {
	err := (*store.Parameters)(cfg).Validate()
	if err != nil {
		return fmt.Errorf("module/header: misconfiguration: %w", err)
	}

	return nil
}
