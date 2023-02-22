package share

import (
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-node/share/availability"
)

var (
	ErrNegativeInterval = errors.New("interval must be positive")
)

type Config struct {
	availability.ShareParameters
	UseShareExchange bool
}

func DefaultConfig() Config {
	return Config{
		ShareParameters:  availability.DefaultShareParameters(),
		UseShareExchange: true,
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	if err := cfg.ShareParameters.Validate(); err != nil {
		return fmt.Errorf("nodebuilder/share: %w", err)
	}

	return nil
}
