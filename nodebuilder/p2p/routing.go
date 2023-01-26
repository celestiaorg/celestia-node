package p2p

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"go.uber.org/fx"
)

// contentRouting constructs nil content routing,
// as for our use-case existing ContentRouting mechanisms, e.g DHT, are unsuitable
func contentRouting(r routing.PeerRouting) routing.ContentRouting {
	return r.(*dht.IpfsDHT)
}

// peerRouting provides constructor for PeerRouting over DHT.
// Basically, this provides a way to discover peer addresses by respecting public keys.
func peerRouting(cfg Config, params routingParams) (routing.PeerRouting, error) {
	opts := []dht.Option{
		dht.Mode(dht.ModeAuto),
		dht.BootstrapPeers(params.Peers...),
		dht.ProtocolPrefix(protocol.ID(fmt.Sprintf("/celestia/%s", params.Net))),
		dht.Datastore(params.DataStore),
		dht.RoutingTableRefreshPeriod(cfg.RoutingTableRefreshPeriod),
	}

	if cfg.Bootstrapper {
		// override options for bootstrapper
		opts = append(opts,
			dht.Mode(dht.ModeServer), // it must accept incoming connections
			dht.BootstrapPeers(),     // no bootstrappers for a bootstrapper ¯\_(ツ)_/¯
		)
	}

	d, err := dht.New(params.Ctx, params.Host, opts...)
	if err != nil {
		return nil, err
	}
	params.Lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return d.Bootstrap(ctx)
		},
		OnStop: func(context.Context) error {
			return d.Close()
		},
	})
	return d, nil
}

type routingParams struct {
	fx.In

	Ctx       context.Context
	Net       Network
	Peers     Bootstrappers
	Lc        fx.Lifecycle
	Host      HostBase
	DataStore datastore.Batching
}
