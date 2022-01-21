package services

import (
	"encoding/hex"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/celestia-node/build"
)

type Config struct {
	// TrustedHash is the Block/Header hash that Nodes use as starting point for header synchronization.
	// Only affects the node once on initial sync.
	TrustedHash string
	// TrustedPeer is the peer we trust to fetch headers from.
	// Note: The trusted does *not* imply Headers are not verified, but trusted as reliable to fetch headers
	// at any moment.
	TrustedPeer string
}

// TODO(@Wondertan): We need to hardcode trustedHash hash and one bootstrap peer as trusted.
func DefaultConfig() Config {
	return Config{
		TrustedHash: "",
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

func (cfg *Config) trustedHash() (tmbytes.HexBytes, error) {
	if cfg.TrustedHash == "" {
		return hex.DecodeString(build.Genesis())
	}
	return hex.DecodeString(cfg.TrustedHash)
}
