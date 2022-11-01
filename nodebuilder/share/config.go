package share

import (
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-node/share/availability"
)

var (
	ErrNegativeInterval = errors.New("interval must be positive")
)

type Config availability.ShareParameters

// DefaultConfig returns the default share module configuration
func DefaultConfig() Config {
	return Config(availability.DefaultShareParameters())
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	err := (*availability.ShareParameters)(cfg).Validate()
	if err != nil {
		return fmt.Errorf("modshare misconfiguration: %s", err)
	}

	return nil
}
