package p2p

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/libs/fxutil"
	nparams "github.com/celestiaorg/celestia-node/params"
)

// ContentRouting constructs nil content routing,
// as for our use-case existing ContentRouting mechanisms, e.g DHT, are unsuitable
func ContentRouting() routing.ContentRouting {
	return &routinghelpers.Null{}
}

// PeerRouting provides constructor for PeerRouting over DHT.
// Basically, this provides a way to discover peer addresses by respecting public keys.
func PeerRouting(cfg Config) func(routingParams) (routing.PeerRouting, error) {
	return func(params routingParams) (routing.PeerRouting, error) {
		bpeers := make([]peer.AddrInfo, len(params.Peers))
		for index := range params.Peers {
			bpeers[index] = *params.Peers[index]
		}
		opts := []dht.Option{
			dht.Mode(dht.ModeAuto),
			dht.BootstrapPeers(bpeers...),
			dht.ProtocolPrefix(protocol.ID(fmt.Sprintf("/celestia/%s", params.Net))),
			dht.Datastore(params.DataStore),
			dht.QueryFilter(dht.PublicQueryFilter),
			dht.RoutingTableFilter(dht.PublicRoutingTableFilter),
			// disable DHT for everything besides peer routing
			dht.DisableValues(),
			dht.DisableProviders(),
		}

		if cfg.Bootstrapper {
			// override options for bootstrapper
			opts = append(opts,
				dht.Mode(dht.ModeServer), // it must accept incoming connections
				dht.BootstrapPeers(),     // no bootstrappers for a bootstrapper ¯\_(ツ)_/¯
			)
		}

		d, err := dht.New(fxutil.WithLifecycle(params.Ctx, params.Lc), params.Host, opts...)
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
}

type routingParams struct {
	fx.In

	Ctx       context.Context
	Net       nparams.Network
	Peers     nparams.BootstrapPeers
	Lc        fx.Lifecycle
	Host      HostBase
	DataStore datastore.Batching
}
