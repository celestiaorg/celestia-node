package p2p

import (
	"context"
	"time"

	"github.com/ipfs/go-datastore"
	connmgri "github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoreds" //nolint:staticcheck //nolint:nolintlint
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// connManagerConfig configures connection manager.
type connManagerConfig struct {
	// Low and High are watermarks governing the number of connections that'll be maintained.
	Low, High int
	// GracePeriod is the amount of time a newly opened connection is given before it becomes subject
	// to pruning.
	GracePeriod time.Duration
}

// defaultConnManagerConfig returns defaults for ConnManagerConfig.
func defaultConnManagerConfig(tp node.Type) connManagerConfig {
	switch tp {
	case node.Light:
		return connManagerConfig{
			Low:         50,
			High:        100,
			GracePeriod: time.Minute,
		}
	case node.Bridge, node.Full:
		return connManagerConfig{
			Low:         800,
			High:        1000,
			GracePeriod: time.Minute,
		}
	default:
		panic("unknown node type")
	}
}

// connectionManager provides a constructor for ConnectionManager.
func connectionManager(cfg *Config, bpeers Bootstrappers) (connmgri.ConnManager, error) {
	fpeers, err := cfg.mutualPeers()
	if err != nil {
		return nil, err
	}
	cm, err := connmgr.NewConnManager(
		cfg.ConnManager.Low,
		cfg.ConnManager.High,
		connmgr.WithGracePeriod(cfg.ConnManager.GracePeriod),
	)
	if err != nil {
		return nil, err
	}
	for _, info := range fpeers {
		cm.Protect(info.ID, "protected-mutual")
	}
	for _, info := range bpeers {
		cm.Protect(info.ID, "protected-bootstrap")
	}

	return cm, nil
}

// connectionGater constructs a BasicConnectionGater.
func connectionGater(ds datastore.Batching) (*conngater.BasicConnectionGater, error) {
	return conngater.NewBasicConnectionGater(ds)
}

// peerStore constructs an on-disk PeerStore.
func peerStore(ctx context.Context, ds datastore.Batching) (peerstore.Peerstore, error) {
	return pstoreds.NewPeerstore(ctx, ds, pstoreds.DefaultOpts())
}
