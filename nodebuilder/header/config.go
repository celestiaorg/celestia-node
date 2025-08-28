package header

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	p2p_exchange "github.com/celestiaorg/go-header/p2p"
	"github.com/celestiaorg/go-header/store"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

// MetricsEnabled will be set during runtime if metrics are enabled on the node.
var MetricsEnabled = false

// Config contains configuration parameters for header retrieval and management.
type Config struct {
	// TrustedHash is the Block/Header hash that Nodes use as starting point for header synchronization.
	// Only affects the node once on initial sync.
	//  TODO: Deprecate or break?
	//   Also depends on whether we want to make pruning a default behavior from the start and for which nodes.
	//   I guess we want to make it default for LNs and the opposite for FNs
	TrustedHash string
	// TrustedPeers are the peers we trust to fetch headers from.
	// Note: The trusted does *not* imply Headers are not verified, but trusted as reliable to fetch
	// headers at any moment.
	TrustedPeers []string

	Store  store.Parameters
	Syncer sync.Parameters

	Server p2p_exchange.ServerParameters
	Client p2p_exchange.ClientParameters `toml:",omitempty"`
}

func DefaultConfig(tp node.Type) Config {
	cfg := Config{
		TrustedHash:  "",
		TrustedPeers: make([]string, 0),
		Store:        store.DefaultParameters(),
		Syncer:       sync.DefaultParameters(),
		Server:       p2p_exchange.DefaultServerParameters(),
		Client:       p2p_exchange.DefaultClientParameters(),
	}

	switch tp {
	case node.Bridge:
		return cfg
	case node.Full:
		return cfg
	case node.Light:
		cfg.Store.StoreCacheSize = 512
		cfg.Store.IndexCacheSize = 2048
		cfg.Store.WriteBatchSize = 512
		return cfg
	default:
		panic("header: invalid node type")
	}
}

func (cfg *Config) trustedPeers(bpeers p2p.Bootstrappers) (infos []peer.AddrInfo, err error) {
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

func (cfg *Config) trustedHash(net p2p.Network) (string, error) {
	if cfg.TrustedHash == "" {
		gen, err := p2p.GenesisFor(net)
		if err != nil {
			return "", err
		}
		return gen, nil
	}
	return cfg.TrustedHash, nil
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate(tp node.Type) error {
	err := cfg.Store.Validate()
	if err != nil {
		return fmt.Errorf("module/header: misconfiguration of store: %w", err)
	}

	err = cfg.Syncer.Validate()
	if err != nil {
		return fmt.Errorf("module/header: misconfiguration of syncer: %w", err)
	}

	err = cfg.Server.Validate()
	if err != nil {
		return fmt.Errorf("module/header: misconfiguration of p2p exchange server: %w", err)
	}

	// we do not create a client for bridge nodes
	if tp == node.Bridge {
		return nil
	}

	err = cfg.Client.Validate()
	if err != nil {
		return fmt.Errorf("module/header: misconfiguration of p2p exchange client: %w", err)
	}

	return nil
}
