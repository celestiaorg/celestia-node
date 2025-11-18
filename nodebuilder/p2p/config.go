package p2p

import (
	"fmt"
	"reflect"

	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// Config combines all configuration fields for P2P subsystem.
type Config struct {
	// ListenAddresses - Addresses to listen to on local NIC.
	ListenAddresses []string
	// AnnounceAddresses - Addresses to be announced/advertised for peers to connect to
	AnnounceAddresses []string
	// NoAnnounceAddresses - Addresses the P2P subsystem may know about, but that should not be
	// announced/advertised, as undialable from WAN
	NoAnnounceAddresses []string
	// MutualPeers are peers which have a bidirectional peering agreement with the configured node.
	// Connections with those peers are protected from being trimmed, dropped or negatively scored.
	// NOTE: Any two peers must bidirectionally configure each other on their MutualPeers field.
	MutualPeers []string
	// PeerExchange configures the node, whether it should share some peers to a pruned peer.
	// This is enabled by default for Bootstrappers.
	PeerExchange bool
	// ConnManager is a configuration tuple for ConnectionManager.
	ConnManager connManagerConfig

	// Allowlist for IPColocation PubSub parameter, a list of string CIDRs
	IPColocationWhitelist []string

	// Disabled disables all P2P networking when true.
	// When disabled, the node will not initialize:
	// - P2P host
	// - DHT
	// - Gossip (PubSub)
	// - Shrex servers/clients
	// - Bitswap servers/clients
	// This is useful for storage-only nodes (e.g., hosted RPC providers).
	// Only supported for Bridge nodes.
	Disabled bool
}

// DefaultConfig returns default configuration for P2P subsystem.
func DefaultConfig(tp node.Type) Config {
	return Config{
		ListenAddresses: []string{
			"/ip4/0.0.0.0/udp/2121/quic-v1/webtransport",
			"/ip6/::/udp/2121/quic-v1/webtransport",
			"/ip4/0.0.0.0/udp/2121/quic-v1",
			"/ip6/::/udp/2121/quic-v1",
			"/ip4/0.0.0.0/udp/2121/webrtc-direct",
			"/ip6/::/udp/2121/webrtc-direct",
			"/ip4/0.0.0.0/tcp/2121",
			"/ip6/::/tcp/2121",
		},
		AnnounceAddresses: []string{},
		NoAnnounceAddresses: []string{
			"/ip4/127.0.0.1/udp/2121/quic-v1/webtransport",
			"/ip4/0.0.0.0/udp/2121/quic-v1/webtransport",
			"/ip6/::/udp/2121/quic-v1/webtransport",
			"/ip4/0.0.0.0/udp/2121/quic-v1",
			"/ip4/127.0.0.1/udp/2121/quic-v1",
			"/ip6/::/udp/2121/quic-v1",
			"/ip4/0.0.0.0/udp/2121/webrtc-direct",
			"/ip4/127.0.0.1/udp/2121/webrtc-direct",
			"/ip6/::/udp/2121/webrtc-direct",
			"/ip4/0.0.0.0/tcp/2121",
			"/ip4/127.0.0.1/tcp/2121",
			"/ip6/::/tcp/2121",
		},
		MutualPeers:  []string{},
		PeerExchange: tp == node.Bridge || tp == node.Full,
		ConnManager:  defaultConnManagerConfig(tp),
	}
}

func (cfg *Config) mutualPeers() (_ []peer.AddrInfo, err error) {
	maddrs := make([]ma.Multiaddr, len(cfg.MutualPeers))
	for i, addr := range cfg.MutualPeers {
		maddrs[i], err = ma.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("failure to parse config.P2P.MutualPeers: %w", err)
		}
	}

	return peer.AddrInfosFromP2pAddrs(maddrs...)
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate(tp node.Type) error {
	if cfg.Disabled {
		// P2P disabled is only supported for Bridge nodes
		if tp != node.Bridge {
			return fmt.Errorf("p2p.disabled is only supported for Bridge nodes")
		}
		// If disabled, other P2P configs are ignored
		return nil
	}
	return nil
}

// ValidateWithShareConfig performs validation that requires Share config.
// This is called separately to avoid circular dependencies.
// shareCfg should be a pointer to share.Config (passed as interface{} to avoid import cycle).
func (cfg *Config) ValidateWithShareConfig(tp node.Type, shareCfg interface{}) error {
	if cfg.Disabled {
		if tp != node.Bridge {
			return fmt.Errorf("p2p.disabled is only supported for Bridge nodes")
		}
		// When P2P is disabled, ODS-only storage should be enabled
		// This ensures storage-only nodes don't waste space storing Q4
		if shareCfg != nil {
			// Use reflection to access StoreODSOnly field to avoid import cycle
			val := reflect.ValueOf(shareCfg)
			if val.Kind() == reflect.Ptr {
				val = val.Elem()
			}
			if val.Kind() == reflect.Struct {
				field := val.FieldByName("StoreODSOnly")
				if field.IsValid() && field.Kind() == reflect.Bool {
					if !field.Bool() {
						return fmt.Errorf("p2p.disabled requires share.store_ods_only to be enabled")
					}
				}
			}
		}
		return nil
	}
	return nil
}

// Upgrade updates the `ListenAddresses` and `NoAnnounceAddresses` to
// include support for websocket connections.
func (cfg *Config) Upgrade() {
	cfg.ListenAddresses = append(
		cfg.ListenAddresses,
		"/ip4/0.0.0.0/tcp/2122/wss",
		"/ip6/::/tcp/2122/wss",
	)
	cfg.NoAnnounceAddresses = append(
		cfg.NoAnnounceAddresses,
		"/ip4/127.0.0.1/tcp/2122/wss",
		"/ip6/::/tcp/2122/wss",
	)
}
