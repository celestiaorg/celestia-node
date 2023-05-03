package share

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	"github.com/celestiaorg/celestia-node/share/p2p/peers"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexeds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"
)

type Config struct {
	Availability light.Parameters
	Discovery    discovery.Parameters

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
		Availability:      light.DefaultParameters(),
		Discovery:         discovery.DefaultParameters(),
		ShrExEDSParams:    shrexeds.DefaultParameters(),
		ShrExNDParams:     shrexnd.DefaultParameters(),
		UseShareExchange:  true,
		PeerManagerParams: peers.DefaultParameters(),
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	err := cfg.Availability.Validate()
	if err != nil {
		return fmt.Errorf("nodebuilder/share: %w", err)
	}

	if err := cfg.Discovery.Validate(); err != nil {
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
