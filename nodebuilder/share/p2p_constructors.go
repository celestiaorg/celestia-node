package share

import (
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"go.uber.org/fx"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modprune "github.com/celestiaorg/celestia-node/nodebuilder/pruner"
	disc "github.com/celestiaorg/celestia-node/share/p2p/discovery"
	"github.com/celestiaorg/celestia-node/share/p2p/peers"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

const (
	// fullNodesTag is the tag used to identify full nodes in the discovery service.
	fullNodesTag = "full"
	// archivalNodesTag is the tag used to identify archival nodes in the
	// discovery service.
	archivalNodesTag = "archival"
)

// TODO @renaynay: rename
func peerComponents(tp node.Type, cfg *Config) fx.Option {
	return fx.Options(
		fullDiscoveryAndPeerManager(tp, cfg),
		archivalDiscoveryAndPeerManager(tp, cfg),
	)
}

// fullDiscoveryAndPeerManager builds the discovery instance and peer manager
// for the `full` tag. Every node type (Light, Full, and Bridge) must discovery
// `full` nodes on the network.
func fullDiscoveryAndPeerManager(tp node.Type, cfg *Config) fx.Option {
	return fx.Provide(
		func(
			lc fx.Lifecycle,
			host host.Host,
			r routing.ContentRouting,
			connGater *conngater.BasicConnectionGater,
			shrexSub *shrexsub.PubSub,
			headerSub libhead.Subscriber[*header.ExtendedHeader],
			// we must ensure Syncer is started before PeerManager
			// so that Syncer registers header validator before PeerManager subscribes to headers
			_ *sync.Syncer[*header.ExtendedHeader],
		) (*peers.Manager, *disc.Discovery, error) {
			var managerOpts []peers.Option
			if tp != node.Bridge {
				// BNs do not need the overhead of shrexsub peer pools as
				// BNs do not sync blocks off the DA network.
				managerOpts = append(managerOpts, peers.WithShrexSubPools(shrexSub, headerSub))
			}

			fullManager, err := peers.NewManager(
				cfg.PeerManagerParams,
				host,
				connGater,
				fullNodesTag,
				managerOpts...,
			)
			if err != nil {
				return nil, nil, err
			}

			discOpts := []disc.Option{disc.WithOnPeersUpdate(fullManager.UpdateNodePool)}

			if tp != node.Light {
				// only FN and BNs should advertise to `full` topic
				discOpts = append(discOpts, disc.WithAdvertise())
			}

			fullDisc, err := disc.NewDiscovery(
				cfg.Discovery,
				host,
				routingdisc.NewRoutingDiscovery(r),
				fullNodesTag,
				discOpts...,
			)
			if err != nil {
				return nil, nil, err
			}
			lc.Append(fx.Hook{
				OnStart: fullDisc.Start,
				OnStop:  fullDisc.Stop,
			})

			return fullManager, fullDisc, nil
		})
}

// archivalDiscoveryAndPeerManager TODO @renaynay
func archivalDiscoveryAndPeerManager(tp node.Type, cfg *Config) fx.Option {
	return fx.Provide(
		func(
			lc fx.Lifecycle,
			pruneCfg *modprune.Config,
			d *disc.Discovery,
			manager *peers.Manager,
			h host.Host,
			r routing.ContentRouting,
			gater *conngater.BasicConnectionGater,
		) (map[string]*peers.Manager, []*disc.Discovery, error) {
			archivalPeerManager, err := peers.NewManager(
				cfg.PeerManagerParams,
				h,
				gater,
				archivalNodesTag,
			)
			if err != nil {
				return nil, nil, err
			}

			discOpts := []disc.Option{disc.WithOnPeersUpdate(archivalPeerManager.UpdateNodePool)}

			if (tp == node.Bridge || tp == node.Full) && !pruneCfg.EnableService {
				discOpts = append(discOpts, disc.WithAdvertise())
			}

			archivalDisc, err := disc.NewDiscovery(
				cfg.Discovery,
				h,
				routingdisc.NewRoutingDiscovery(r),
				archivalNodesTag,
				discOpts...,
			)
			if err != nil {
				return nil, nil, err
			}
			lc.Append(fx.Hook{
				OnStart: archivalDisc.Start,
				OnStop:  archivalDisc.Stop,
			})

			managers := map[string]*peers.Manager{fullNodesTag: manager, archivalNodesTag: archivalPeerManager}
			return managers, []*disc.Discovery{d, archivalDisc}, nil
		})
}
