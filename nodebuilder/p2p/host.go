package p2p

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p"
	p2pconfig "github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/crypto"
	hst "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/routing"
	routedhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// routedHost constructs a wrapped Host that may fallback to address discovery,
// if any top-level operation on the Host is provided with PeerID(Hash(PbK)) only.
func routedHost(base HostBase, r routing.PeerRouting) hst.Host {
	return routedhost.Wrap(base, r)
}

// host returns constructor for Host.
func host(params hostParams) (HostBase, error) {
	opts := []libp2p.Option{
		libp2p.NoListenAddrs, // do not listen automatically
		libp2p.AddrsFactory(params.AddrF),
		libp2p.Identity(params.Key),
		libp2p.Peerstore(params.PStore),
		libp2p.ConnectionManager(params.ConnMngr),
		libp2p.ConnectionGater(params.ConnGater),
		libp2p.UserAgent(fmt.Sprintf("celestia-%s", params.Net)),
		libp2p.NATPortMap(), // enables upnp
		libp2p.DisableRelay(),
		libp2p.BandwidthReporter(params.Bandwidth),
		libp2p.ResourceManager(params.ResourceManager),
		// to clearly define what defaults we rely upon
		libp2p.DefaultSecurity,
		libp2p.DefaultTransports,
		libp2p.DefaultMuxers,
	}

	// All node types except light (bridge, full) will enable NATService
	if params.Tp != node.Light {
		opts = append(opts, libp2p.EnableNATService())
	}

	h, err := libp2p.NewWithoutDefaults(opts...)
	if err != nil {
		return nil, err
	}

	params.Lc.Append(fx.Hook{OnStop: func(context.Context) error {
		return h.Close()
	}})

	return h, nil
}

type HostBase hst.Host

type hostParams struct {
	fx.In

	Net             Network
	Lc              fx.Lifecycle
	ID              peer.ID
	Key             crypto.PrivKey
	AddrF           p2pconfig.AddrsFactory
	PStore          peerstore.Peerstore
	ConnMngr        connmgr.ConnManager
	ConnGater       *conngater.BasicConnectionGater
	Bandwidth       *metrics.BandwidthCounter
	ResourceManager network.ResourceManager

	Tp node.Type
}
