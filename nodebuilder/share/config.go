package share

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/share/shwap/p2p/discovery"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/peers"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexeds"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexnd"
	"github.com/celestiaorg/celestia-node/store"
)

const (
	defaultBlockstoreCacheSize = 128
)

type Config struct {
	// EDSStoreParams sets eds store configuration parameters
	EDSStoreParams      *store.Parameters
	BlockStoreCacheSize uint

	UseShareExchange bool
	// ShrExEDSParams sets shrexeds client and server configuration parameters
	ShrExEDSParams *shrexeds.Parameters
	// ShrExNDParams sets shrexnd client and server configuration parameters
	ShrExNDParams *shrexnd.Parameters
	// PeerManagerParams sets peer-manager configuration parameters
	PeerManagerParams *peers.Parameters

	Discovery *discovery.Parameters
}

func DefaultConfig() Config {
	cfg := Config{
		EDSStoreParams:      store.DefaultParameters(),
		BlockStoreCacheSize: defaultBlockstoreCacheSize,
		Discovery:           discovery.DefaultParameters(),
		ShrExEDSParams:      shrexeds.DefaultParameters(),
		ShrExNDParams:       shrexnd.DefaultParameters(),
		UseShareExchange:    true,
		PeerManagerParams:   peers.DefaultParameters(),
	}

	return cfg
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	if err := cfg.Discovery.Validate(); err != nil {
		return fmt.Errorf("discovery: %w", err)
	}

	if err := cfg.ShrExNDParams.Validate(); err != nil {
		return fmt.Errorf("shrexnd: %w", err)
	}

	if err := cfg.ShrExEDSParams.Validate(); err != nil {
		return fmt.Errorf("shrexeds: %w", err)
	}

	if err := cfg.PeerManagerParams.Validate(); err != nil {
		return fmt.Errorf("peer manager: %w", err)
	}

	if err := cfg.EDSStoreParams.Validate(); err != nil {
		return fmt.Errorf("eds store: %w", err)
	}
	return nil
}
