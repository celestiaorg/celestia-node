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

func (cfg *Config) trustedPeers(net params.Network) (infos []*peer.AddrInfo, err error) {
	if len(cfg.TrustedPeers) == 0 {
		log.Info("No trusted peers in config, initializing with default bootstrappers as trusted peers")
		cfg.TrustedPeers, err = params.BootstrappersFor(net)
		if err != nil {
			return
		}
	}

	infos = make([]*peer.AddrInfo, len(cfg.TrustedPeers))
	for i, tpeer := range cfg.TrustedPeers {
		ma, err := multiaddr.NewMultiaddr(tpeer)
		if err != nil {
			return nil, err
		}
		infos[i], err = peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			return nil, err
		}
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
