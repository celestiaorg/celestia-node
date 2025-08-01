package share

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/discovery"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/peers"
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
	// Shrex sets client and server configuration parameters of the shrex protocol
	ShrexClient *shrex.ClientParams
	ShrexServer *shrex.ServerParams
	// PeerManagerParams sets peer-manager configuration parameters
	PeerManagerParams *peers.Parameters

	LightAvailability *light.Parameters `toml:",omitempty"`
	Discovery         *discovery.Parameters
}

func DefaultConfig(tp node.Type) Config {
	cfg := Config{
		EDSStoreParams:      store.DefaultParameters(),
		BlockStoreCacheSize: defaultBlockstoreCacheSize,
		Discovery:           discovery.DefaultParameters(),
		ShrexClient:         shrex.DefaultClientParameters(),
		ShrexServer:         shrex.DefaultServerParameters(),
		UseShareExchange:    true,
		PeerManagerParams:   peers.DefaultParameters(),
	}

	if tp == node.Light {
		cfg.LightAvailability = light.DefaultParameters()
	}

	return cfg
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate(tp node.Type) error {
	if tp == node.Light {
		if err := cfg.LightAvailability.Validate(); err != nil {
			return fmt.Errorf("nodebuilder/share: %w", err)
		}
	}

	if err := cfg.Discovery.Validate(); err != nil {
		return fmt.Errorf("discovery: %w", err)
	}

	if err := cfg.ShrexClient.Validate(); err != nil {
		return fmt.Errorf("shrex-client: %w", err)
	}

	if err := cfg.ShrexServer.Validate(); err != nil {
		return fmt.Errorf("shrex-server: %w", err)
	}

	if err := cfg.PeerManagerParams.Validate(); err != nil {
		return fmt.Errorf("peer manager: %w", err)
	}

	if err := cfg.EDSStoreParams.Validate(); err != nil {
		return fmt.Errorf("eds store: %w", err)
	}
	return nil
}
