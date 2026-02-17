package p2p

import (
	"fmt"

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
		PeerExchange: tp == node.Bridge,
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
