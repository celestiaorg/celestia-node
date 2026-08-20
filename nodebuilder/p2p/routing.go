package p2p

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/routing"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/discovery"
)

func newDHT(
	ctx context.Context,
	lc fx.Lifecycle,
	cfg *Config,
	tp node.Type,
	network Network,
	bootstrappers Bootstrappers,
	host HostBase,
	dataStore datastore.Batching,
) (*dht.IpfsDHT, error) {
	var mode dht.ModeOpt
	switch tp {
	case node.Light:
		mode = dht.ModeClient
	case node.Bridge:
		mode = dht.ModeServer
	default:
		return nil, fmt.Errorf("unsupported node type: %s", tp)
	}

	// no bootstrappers for a bootstrapper ¯\_(ツ)_/¯
	// otherwise dht.Bootstrap(OnStart hook) will deadlock
	if isBootstrapper() {
		bootstrappers = nil
	}

	dht, err := discovery.NewDHT(ctx, network.String(), bootstrappers, host, dataStore, mode)
	if err != nil {
		return nil, err
	}
	// Register cleanup first so Fx runs it if Listen or Bootstrap fails.
	lc.Append(fx.Hook{OnStop: func(context.Context) error {
		return dht.Close()
	}})
	lc.Append(fx.Hook{OnStart: func(ctx context.Context) error {
		if err := Listen(cfg)(host); err != nil {
			return err
		}
		return dht.Bootstrap(ctx)
	}})
	return dht, nil
}

func peerRouting(dht *dht.IpfsDHT) routing.PeerRouting {
	return dht
}
