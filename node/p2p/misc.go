package p2p

import (
	"time"

	"github.com/ipfs/go-datastore"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	coreconnmgr "github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
)

type ConnManagerConfig struct {
	Low, High   int
	GracePeriod time.Duration
}

func DefaultConnManagerConfig() *ConnManagerConfig {
	return &ConnManagerConfig{
		Low:         50,
		High:        100,
		GracePeriod: time.Minute,
	}
}

func ConnectionManager(cfg *Config) func() (coreconnmgr.ConnManager, error) {
	return func() (coreconnmgr.ConnManager, error) {
		fpeers, err := cfg.friendPeers()
		if err != nil {
			return nil, err
		}

		bpeers, err := cfg.bootstrapPeers()
		if err != nil {
			return nil, err
		}

		cm := connmgr.NewConnManager(cfg.ConnManager.Low, cfg.ConnManager.High, cfg.ConnManager.GracePeriod)
		for _, info := range fpeers {
			cm.Protect(info.ID, "69protected69_config")
		}
		for _, info := range bpeers {
			cm.Protect(info.ID, "69protected69_bootstrap")
		}

		return cm, nil
	}
}

func ConnectionGater(ds datastore.Batching) (coreconnmgr.ConnectionGater, error) {
	return conngater.NewBasicConnectionGater(ds)
}

func PeerStore() peerstore.Peerstore {
	return pstoremem.NewPeerstore()
}
