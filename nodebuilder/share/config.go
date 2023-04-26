package share

import (
	disc "github.com/celestiaorg/celestia-node/share/availability/discovery"
	"github.com/celestiaorg/celestia-node/share/p2p/peers"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexeds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"
)

type Config struct {
	// UseShareExchange is a flag toggling the usage of shrex protocols for blocksync.
	// NOTE: This config variable only has an effect on full and bridge nodes.
	UseShareExchange bool
	// ShrExEDSParams sets shrexeds client and server configuration parameters
	ShrExEDSParams *shrexeds.Parameters
	// ShrExNDParams sets shrexnd client and server configuration parameters
	ShrExNDParams *shrexnd.Parameters
	// PeerManagerParams sets peer-manager configuration parameters
	PeerManagerParams peers.Parameters
	// Discovery sets peer discovery configuration parameters.
	Discovery disc.Parameters
}

func DefaultConfig() Config {
	return Config{
		UseShareExchange:  true,
		ShrExEDSParams:    shrexeds.DefaultParameters(),
		ShrExNDParams:     shrexnd.DefaultParameters(),
		PeerManagerParams: peers.DefaultParameters(),
		Discovery:         disc.DefaultParameters(),
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	if err := cfg.ShrExNDParams.Validate(); err != nil {
		return err
	}
	if err := cfg.ShrExEDSParams.Validate(); err != nil {
		return err
	}
	return cfg.PeerManagerParams.Validate()
}
