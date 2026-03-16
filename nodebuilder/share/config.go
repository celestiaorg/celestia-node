package share

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share"
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
	UseBitswap       bool

	// Shrex sets client and server configuration parameters of the shrex protocol
	ShrexClient *shrex.ClientParams
	ShrexServer *shrex.ServerParams
	// PeerManagerParams sets peer-manager configuration parameters
	PeerManagerParams *peers.Parameters

	LightAvailability *light.Parameters `toml:",omitempty"`
	Discovery         *discovery.Parameters

	// RDA grid configuration
	RDAEnabled           bool                 `toml:",omitempty"`
	RDAGridDimensions    share.GridDimensions `toml:",omitempty"`
	RDAExpectedNodeCount int                  `toml:",omitempty"`
	RDAFilterPolicy      share.FilterPolicy   `toml:",omitempty"`
	RDADetailedLogging   bool                 `toml:",omitempty"`

	// RDADiscoveryEnabled controls whether DHT-based grid peer discovery is
	// active. Set false in isolated test environments (mocknet/swamp).
	RDADiscoveryEnabled bool `toml:",omitempty"`

	// RDABootstrapPeers is a list of multiaddr strings for known stable peers
	// (typically bridge nodes) that a new node contacts first to enter the
	// DHT and find its grid neighbors.
	// Example: ["/ip4/1.2.3.4/tcp/2121/p2p/12D3KooW..."]
	RDABootstrapPeers []string `toml:",omitempty"`

	// RDAPeerBootstraps is a list of other bootstrap servers for peer info sync.
	// Only used when this node is a bootstrap server itself.
	// Example: ["/dns4/bootstrap2/tcp/2121/p2p/12D3KooW..."]
	RDAPeerBootstraps []string `toml:",omitempty"`

	// RDAUseSubnetDiscovery enables the RDA paper-based subnet discovery protocol.
	// When true, nodes discover peers via subnet announcements/gossip instead of DHT.
	// This is the preferred method according to the RDA paper.
	RDAUseSubnetDiscovery bool `toml:",omitempty"`

	// RDASubnetDiscoveryDelay is the delay before pulling full membership list
	// after announcing to a subnet. Implements "delayed pull" from the paper.
	// Default: 4 seconds (~4 rounds).
	RDASubnetDiscoveryDelay string `toml:",omitempty"`

	// CDA configuration
	// CDAEnabled toggles the Coded Distributed Array pipeline. When false (default),
	// the node behaves purely as an RDA node using full-share replication.
	CDAEnabled bool `toml:",omitempty"`
	// CDAK is the target coding rate k used for RLNC-based fragmentation.
	// Small k (e.g. 16 or 32) keeps decoding inexpensive.
	CDAK int `toml:",omitempty"`
	// CDAUseRDAFallback allows falling back to the legacy RDA path (full shares)
	// when CDA fragments are unavailable or decoding fails.
	CDAUseRDAFallback bool `toml:",omitempty"`

	// CDABuffer is the per-share storage headroom beyond k (hard cap = k + buffer).
	// Recommended default is 4 for k=16 to tolerate linear dependence while staying small.
	CDABuffer int `toml:",omitempty"`
}

func DefaultConfig(tp node.Type) Config {
	cfg := Config{
		EDSStoreParams:          store.DefaultParameters(),
		BlockStoreCacheSize:     defaultBlockstoreCacheSize,
		Discovery:               discovery.DefaultParameters(),
		ShrexClient:             shrex.DefaultClientParameters(),
		ShrexServer:             shrex.DefaultServerParameters(),
		UseShareExchange:        true,
		UseBitswap:              true,
		PeerManagerParams:       peers.DefaultParameters(),
		RDAEnabled:              true,
		RDAGridDimensions:       share.GridDimensions{Rows: 128, Cols: 128},
		RDAExpectedNodeCount:    16384,
		RDAFilterPolicy:         share.DefaultFilterPolicy(),
		RDADetailedLogging:      false,
		RDADiscoveryEnabled:     true,
		RDABootstrapPeers:       nil,  // populated by operator / network config
		RDAPeerBootstraps:       nil,  // populated by operator / network config
		RDAUseSubnetDiscovery:   true, // Use subnet protocol by default (per paper)
		RDASubnetDiscoveryDelay: "4s", // 4 seconds default (~4 rounds)

		CDAEnabled:         false,
		CDAK:               16,
		CDAUseRDAFallback:  true,
		CDABuffer:          4,
	}

	if tp == node.Light {
		cfg.LightAvailability = light.DefaultParameters()
	}

	return cfg
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate(tp node.Type) error {
	if tp == node.Light {
		if cfg.LightAvailability == nil {
			cfg.LightAvailability = light.DefaultParameters()
		}
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
