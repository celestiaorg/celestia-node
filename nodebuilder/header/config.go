package header

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	p2p_exchange "github.com/celestiaorg/go-header/p2p"
	"github.com/celestiaorg/go-header/store"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share/availability"
)

const trustingPeriod = 7 * 24 * time.Hour // Ref: CIP-036

// MetricsEnabled will be set during runtime if metrics are enabled on the node.
var MetricsEnabled = false

// Config contains configuration parameters for header retrieval and management.
type Config struct {
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
		TrustedPeers: make([]string, 0),
		Store:        store.DefaultParameters(),
		Syncer:       sync.DefaultParameters(),
		Server:       p2p_exchange.DefaultServerParameters(),
		Client:       p2p_exchange.DefaultClientParameters(),
	}

	switch tp {
	case node.Full, node.Bridge:
		cfg.Store.StoreCacheSize = 2048
		cfg.Store.IndexCacheSize = 4096

		cfg.Syncer.PruningWindow = 0 // reset pruning window to zero
		return cfg
	case node.Light:
		cfg.Store.WriteBatchSize = 16

		cfg.Syncer.PruningWindow = availability.StorageWindow
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
	return infos, nil
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate(tp node.Type) error {
	switch tp {
	case node.Full, node.Bridge:
		if cfg.Syncer.SyncFromHash != "" || cfg.Syncer.SyncFromHeight != 0 || cfg.Syncer.PruningWindow != 0 {
			return fmt.Errorf(
				"module/header: Syncer.SyncFromHash/Syncer.SyncFromHeight/Syncer.PruningWindow must not be set for FN/BN nodes" +
					"until https://github.com/celestiaorg/go-header/issues/333 is completed. Full and Bridge must sync all the headers until then",
			)
		}
	case node.Light:
		if cfg.Syncer.PruningWindow < availability.StorageWindow {
			// TODO(@Wondertan): Technically, a LN may break this restriction by setting SyncFromHeight/Hash to a header
			//  that is closer to Head than StorageWindow. Consider putting efforts into catching this too.
			return fmt.Errorf("module/header: Syncer.PruningWindow must not be less then sampling storage window (%s)",
				availability.StorageWindow)
		}
	default:
		panic("invalid node type")
	}

	return nil
}
