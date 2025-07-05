package share

import (
	dht "github.com/libp2p/go-libp2p-kad-dht"
	p2pdisc "github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"go.uber.org/fx"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/discovery"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/peers"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
)

const (
	// fullNodesTag is the tag used to identify full nodes in the discovery service.
	fullNodesTag = "full"
	// archivalNodesTag is the tag used to identify archival nodes in the
	// discovery service.
	archivalNodesTag = "archival"

	// protocolVersion is a prefix for all tags used in discovery. It is bumped when
	// there are protocol breaking changes to prevent new software version to discover older versions.
	protocolVersion = "v0.1.0"
)

func peerManagementComponents(tp node.Type, cfg *Config) fx.Option {
	return fx.Options(
		fx.Provide(routingDiscovery),
		fullDiscoveryAndPeerManager(tp, cfg),
		archivalDiscoveryAndPeerManager(cfg),
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
			connGater *conngater.BasicConnectionGater,
			disc p2pdisc.Discovery,
			shrexSub *shrexsub.PubSub,
			headerSub libhead.Subscriber[*header.ExtendedHeader],
			// we must ensure Syncer is started before PeerManager
			// so that Syncer registers header validator before PeerManager subscribes to headers
			_ *sync.Syncer[*header.ExtendedHeader],
		) (*peers.Manager, *discovery.Discovery, error) {
			// If any required dependencies are nil (p2p disabled), return nils
			if host == nil || disc == nil || connGater == nil {
				return nil, nil, nil
			}
			
			var managerOpts []peers.Option
			if tp != node.Bridge && shrexSub != nil {
				// BNs do not need the overhead of shrexsub peer pools as
				// BNs do not sync blocks off the DA network.
				managerOpts = append(managerOpts, peers.WithShrexSubPools(shrexSub, headerSub))
			}

			fullManager, err := peers.NewManager(
				*cfg.PeerManagerParams,
				host,
				connGater,
				fullNodesTag,
				managerOpts...,
			)
			if err != nil {
				return nil, nil, err
			}

			discOpts := []discovery.Option{discovery.WithOnPeersUpdate(fullManager.UpdateNodePool)}

			if tp != node.Light {
				// only FN and BNs should advertise to `full` topic
				discOpts = append(discOpts, discovery.WithAdvertise())
			}

			fullDisc, err := discovery.NewDiscovery(
				cfg.Discovery,
				host,
				disc,
				fullNodesTag,
				protocolVersion,
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

// archivalDiscoveryAndPeerManager builds the discovery instance and peer manager
// for discovering and managing peers advertising on the `archival` tag
func archivalDiscoveryAndPeerManager(cfg *Config) fx.Option {
	return fx.Provide(
		func(
			lc fx.Lifecycle,
			fullDisc *discovery.Discovery,
			fullManager *peers.Manager,
			h host.Host,
			disc p2pdisc.Discovery,
			gater *conngater.BasicConnectionGater,
			discOpt discovery.Option,
		) (map[string]*peers.Manager, []*discovery.Discovery, error) {
			// If required dependencies are nil (p2p disabled), return empty collections
			if fullDisc == nil || fullManager == nil || h == nil || disc == nil || gater == nil {
				return map[string]*peers.Manager{}, []*discovery.Discovery{}, nil
			}
			
			archivalPeerManager, err := peers.NewManager(
				*cfg.PeerManagerParams,
				h,
				gater,
				archivalNodesTag,
			)
			if err != nil {
				return nil, nil, err
			}

			discOpts := []discovery.Option{discovery.WithOnPeersUpdate(archivalPeerManager.UpdateNodePool)}
			if discOpt != nil {
				discOpts = append(discOpts, discOpt)
			}

			archivalDisc, err := discovery.NewDiscovery(
				cfg.Discovery,
				h,
				disc,
				archivalNodesTag,
				protocolVersion,
				discOpts...,
			)
			if err != nil {
				return nil, nil, err
			}
			lc.Append(fx.Hook{
				OnStart: archivalDisc.Start,
				OnStop:  archivalDisc.Stop,
			})

			managers := map[string]*peers.Manager{fullNodesTag: fullManager, archivalNodesTag: archivalPeerManager}
			return managers, []*discovery.Discovery{fullDisc, archivalDisc}, nil
		})
}

func routingDiscovery(dht *dht.IpfsDHT) p2pdisc.Discovery {
	if dht == nil {
		return nil
	}
	return routingdisc.NewRoutingDiscovery(dht)
}
