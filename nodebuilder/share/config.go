package share

import (
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-node/share/availability/cache"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	"github.com/celestiaorg/celestia-node/share/p2p/peers"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexeds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"
)

var (
	ErrNegativeInterval = errors.New("interval must be positive")
)

// Parameters is the aggregation of all configuration parameters of the the share module
type Parameters struct {
	Light     light.Parameters
	Cache     cache.Parameters
	Discovery discovery.Parameters
}

type Config struct {
	Parameters
	UseShareExchange bool
	// ShrExEDSParams sets shrexeds client and server configuration parameters
	ShrExEDSParams *shrexeds.Parameters
	// ShrExNDParams sets shrexnd client and server configuration parameters
	ShrExNDParams *shrexnd.Parameters
	// PeerManagerParams sets peer-manager configuration parameters
	PeerManagerParams peers.Parameters
}

// TODO: Remove share/availability/options.go and reorg configs here
func DefaultConfig() Config {
	return Config{
		Parameters: Parameters{
			Light:     light.DefaultParameters(),
			Cache:     cache.DefaultParameters(),
			Discovery: discovery.DefaultParameters(),
		},
		ShrExEDSParams:    shrexeds.DefaultParameters(),
		ShrExNDParams:     shrexnd.DefaultParameters(),
		UseShareExchange:  true,
		PeerManagerParams: peers.DefaultParameters(),
	}
}

// Validate performs basic validation of the share Parameters
func (p *Parameters) Validate() error {
	err := p.Light.Validate()
	if err != nil {
		return err
	}

	err = p.Cache.Validate()
	if err != nil {
		return err
	}

	return p.Discovery.Validate()
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	if err := cfg.Parameters.Validate(); err != nil {
		return fmt.Errorf("nodebuilder/share: %w", err)
	}

	if err := cfg.ShrExNDParams.Validate(); err != nil {
		return fmt.Errorf("nodebuilder/share: %w", err)
	}

	if err := cfg.ShrExEDSParams.Validate(); err != nil {
		return fmt.Errorf("nodebuilder/share: %w", err)
	}

	if err := cfg.PeerManagerParams.Validate(); err != nil {
		return fmt.Errorf("nodebuilder/share: %w", err)
	}

	return nil
}
