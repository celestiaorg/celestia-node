package p2p

import (
	"time"

	"github.com/ipfs/go-datastore"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	coreconnmgr "github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"

	"github.com/celestiaorg/celestia-node/params"
)

// ConnManagerConfig configures connection manager.
type ConnManagerConfig struct {
	// Low and High are watermarks governing the number of connections that'll be maintained.
	Low, High int
	// GracePeriod is the amount of time a newly opened connection is given before it becomes subject to pruning.
	GracePeriod time.Duration
}

// DefaultConnManagerConfig returns defaults for ConnManagerConfig.
func DefaultConnManagerConfig() ConnManagerConfig {
	return ConnManagerConfig{
		Low:         50,
		High:        100,
		GracePeriod: time.Minute,
	}
}

// ConnectionManager provides a constructor for ConnectionManager.
func ConnectionManager(cfg Config) func() (coreconnmgr.ConnManager, error) {
	return func() (coreconnmgr.ConnManager, error) {
		fpeers, err := cfg.mutualPeers()
		if err != nil {
			return nil, err
		}
		bpeers := params.BootstrappersInfos()

		cm := connmgr.NewConnManager(cfg.ConnManager.Low, cfg.ConnManager.High, cfg.ConnManager.GracePeriod)
		for _, info := range fpeers {
			cm.Protect(info.ID, "protected-mutual")
		}
		for _, info := range bpeers {
			cm.Protect(info.ID, "protected-bootstrap")
		}

		return cm, nil
	}
}

// ConnectionGater constructs a ConnectionGater.
func ConnectionGater(ds datastore.Batching) (coreconnmgr.ConnectionGater, error) {
	return conngater.NewBasicConnectionGater(ds)
}

// PeerStore constructs a PeerStore.
func PeerStore() peerstore.Peerstore {
	return pstoremem.NewPeerstore()
}
