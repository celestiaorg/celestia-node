package share

import (
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-node/share/availability/cache"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	"github.com/celestiaorg/celestia-node/share/availability/light"

	"github.com/celestiaorg/celestia-node/share/p2p/shrexeds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"
)

var (
	ErrNegativeInterval = errors.New("interval must be positive")
)

// // ShareParameters is the aggergation of all configuration parameters of the the share module
type ShareParameters struct {
	// FullAvailabilityParameters
	Light     light.Parameters
	Cache     cache.Parameters
	Discovery discovery.Parameters
}

type Config struct {
	ShareParameters
	UseShareExchange bool
	// ShrExEDSParams sets shrexeds client and server configuration parameters
	ShrExEDSParams *shrexeds.Parameters
	// ShrExNDParams sets shrexnd client and server configuration parameters
	ShrExNDParams *shrexnd.Parameters
}

// TODO: Remove share/availability/options.go and reorg configs here
func DefaultConfig() Config {
	return Config{
		ShareParameters: ShareParameters{
			Light:     light.DefaultParameters(),
			Cache:     cache.DefaultParameters(),
			Discovery: discovery.DefaultParameters(),
		},
		ShrExEDSParams:   shrexeds.DefaultParameters(),
		ShrExNDParams:    shrexnd.DefaultParameters(),
		UseShareExchange: true,
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	err := cfg.ShareParameters.Light.Validate()
	if err != nil {
		return fmt.Errorf("nodebuilder/share: %w", err)
	}

	err = cfg.ShareParameters.Cache.Validate()
	if err != nil {
		return fmt.Errorf("nodebuilder/share: %w", err)
	}

	err = cfg.ShareParameters.Discovery.Validate()
	if err != nil {
		return fmt.Errorf("nodebuilder/share: %w", err)
	}

	if err := cfg.ShrExNDParams.Validate(); err != nil {
		return fmt.Errorf("nodebuilder/share: %w", err)
	}

	if err := cfg.ShrExEDSParams.Validate(); err != nil {
		return fmt.Errorf("nodebuilder/share: %w", err)
	}

	return nil
}
