package share

import (
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-node/share/availability"

	"github.com/celestiaorg/celestia-node/share/p2p/shrexeds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"
)

var (
	ErrNegativeInterval = errors.New("interval must be positive")
)

type Config struct {
	availability.ShareParameters
	UseShareExchange bool
	// ShrExEDSParams sets shrexeds client and server configuration parameters
	ShrExEDSParams *shrexeds.Parameters
	// ShrExNDParams sets shrexnd client and server configuration parameters
	ShrExNDParams *shrexnd.Parameters
}

func DefaultConfig() Config {
	return Config{
		ShareParameters:  availability.DefaultShareParameters(),
		UseShareExchange: true,
		ShrExEDSParams:   shrexeds.DefaultParameters(),
		ShrExNDParams:    shrexnd.DefaultParameters(),
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	if err := cfg.ShareParameters.Validate(); err != nil {
		return fmt.Errorf("nodebuilder/share: %w", err)
	}
	if err := cfg.ShrExNDParams.Validate(); err != nil {
		return err
	}
	return cfg.ShrExEDSParams.Validate()
}
