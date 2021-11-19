package services

import (
	"encoding/hex"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

type Config struct {
	// GenesisHash is the Block/Header hash that Nodes use as starting point for header synchronization.
	// Only affects the node once on initial sync.
	GenesisHash string
	// TrustedPeer is the peer we trust to fetch headers from.
	// Note: The trusted does *not* imply Headers are not verified, but trusted as reliable to fetch headers
	// at any moment.
	TrustedPeer string
}

// TODO(@Wondertan): We need to hardcode genesis hash and one bootstrap peer as trusted.
func DefaultConfig() Config {
	return Config{
		GenesisHash: "",
		TrustedPeer: "",
	}
}

func (cfg *Config) trustedPeer() (*peer.AddrInfo, error) {
	if cfg.TrustedPeer == "" {
		return &peer.AddrInfo{}, nil
	}

	ma, err := multiaddr.NewMultiaddr(cfg.TrustedPeer)
	if err != nil {
		return nil, err
	}

	return peer.AddrInfoFromP2pAddr(ma)
}

func (cfg *Config) genesis() (tmbytes.HexBytes, error) {
	return hex.DecodeString(cfg.GenesisHash)
}
