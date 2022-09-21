package header

import (
	"encoding/hex"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/celestia-node/params"
)

// Config contains configuration parameters for header retrieval and management.
type Config struct {
	// TrustedHash is the Block/Header hash that Nodes use as starting point for header synchronization.
	// Only affects the node once on initial sync.
	TrustedHash string
	// TrustedPeers are the peers we trust to fetch headers from.
	// Note: The trusted does *not* imply Headers are not verified, but trusted as reliable to fetch headers
	// at any moment.
	TrustedPeers []string
}

func DefaultConfig() Config {
	return Config{
		TrustedHash:  "",
		TrustedPeers: make([]string, 0),
	}
}

func (cfg *Config) trustedPeers(bpeers params.Bootstrappers) (infos []peer.AddrInfo, err error) {
	if len(cfg.TrustedPeers) == 0 {
		log.Infof("No trusted peers in config, initializing with default bootstrappers as trusted peers")
		return bpeers, nil
	}

	infos = make([]peer.AddrInfo, len(cfg.TrustedPeers))
	for i, tpeer := range cfg.TrustedPeers {
		ma, err := multiaddr.NewMultiaddr(tpeer)
		if err != nil {
			return nil, err
		}
		p, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			return nil, err
		}
		infos[i] = *p
	}
	return
}

func (cfg *Config) trustedHash(net params.Network) (tmbytes.HexBytes, error) {
	if cfg.TrustedHash == "" {
		gen, err := params.GenesisFor(net)
		if err != nil {
			return nil, err
		}
		return hex.DecodeString(gen)
	}
	return hex.DecodeString(cfg.TrustedHash)
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	return nil
}
