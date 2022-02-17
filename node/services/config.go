package services

import (
	"encoding/hex"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/celestia-node/params"
)

var log = logging.Logger("node/services")

type Config struct {
	// TrustedHash is the Block/Header hash that Nodes use as starting point for header synchronization.
	// Only affects the node once on initial sync.
	TrustedHash string
	// TrustedPeers is the peer we trust to fetch headers from.
	// Note: The trusted does *not* imply Headers are not verified, but trusted as reliable to fetch headers
	// at any moment.
	TrustedPeers []string
}

// TODO(@Wondertan): We need to hardcode trustedHash hash and one bootstrap peer as trusted.
func DefaultConfig() Config {
	return Config{
		TrustedHash:  "",
		TrustedPeers: make([]string, 0),
	}
}

func (cfg *Config) trustedPeers() ([]*peer.AddrInfo, error) {
	if len(cfg.TrustedPeers) == 0 {
		log.Warn("No Trusted Peers provided. Get default.")
		cfg.TrustedPeers = params.Bootstrappers()
	}

	addrInfos := make([]*peer.AddrInfo, len(cfg.TrustedPeers))
	for index, tpeer := range cfg.TrustedPeers {
		ma, err := multiaddr.NewMultiaddr(tpeer)
		if err != nil {
			return nil, err
		}
		addrInfo, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			return nil, err
		}
		addrInfos[index] = addrInfo
	}

	return addrInfos, nil
}

func (cfg *Config) trustedHash() (tmbytes.HexBytes, error) {
	if cfg.TrustedHash == "" {
		return hex.DecodeString(params.Genesis())
	}
	return hex.DecodeString(cfg.TrustedHash)
}
