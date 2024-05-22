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
		discoveryManager(),
		fullDiscoveryAndPeerManager(tp, cfg),
		archivalDiscoveryAndPeerManager(tp, cfg),
	)
}

func discoveryManager() fx.Option {
	return fx.Options(
		fx.Invoke(func(*disc.Manager) {}), // quirk in FX
		fx.Provide(func(
			lc fx.Lifecycle,
			discs map[string]*disc.Discovery,
		) *disc.Manager {
			manager := disc.NewManager(discs)
			lc.Append(
				fx.Hook{
					OnStart: manager.Start,
					OnStop:  manager.Stop,
				},
			)
			return manager
		}),
	)
}

// fullDiscoveryAndPeerManager builds the discovery instance and peer manager
// for the `full` tag. Every node type (Light, Full, and Bridge) must discovery
// `full` nodes on the network.
func fullDiscoveryAndPeerManager(tp node.Type, cfg *Config) fx.Option {
	return fx.Provide(
		func(
			host host.Host,
			r routing.ContentRouting,
			connGater *conngater.BasicConnectionGater,
			shrexSub *shrexsub.PubSub,
			headerSub libhead.Subscriber[*header.ExtendedHeader],
			// we must ensure Syncer is started before PeerManager
			// so that Syncer registers header validator before PeerManager subscribes to headers
			_ *sync.Syncer[*header.ExtendedHeader],
		) (*peers.Manager, *disc.Discovery, error) {
			managerOpts := []peers.Option{peers.WithTag(fullNodesTag)}
			if tp != node.Bridge {
				// BNs do not need the overhead of shrexsub peer pools as
				// BNs do not sync blocks off the DA network.
				managerOpts = append(managerOpts, peers.WithShrexSubPools(shrexSub, headerSub))
			}

			// TODO @renaynay: where is this manager's lifecycle managed?
			fullManager, err := peers.NewManager(
				cfg.PeerManagerParams,
				host,
				connGater,
				managerOpts...,
			)
			if err != nil {
				return nil, nil, err
			}

			fullDisc, err := disc.NewDiscovery(
				cfg.Discovery,
				host,
				routingdisc.NewRoutingDiscovery(r),
				fullNodesTag,
				disc.WithOnPeersUpdate(fullManager.UpdateNodePool),
			)
			if err != nil {
				return nil, nil, err
			}

			return fullManager, fullDisc, nil
		})
}

// TODO @renaynay: doc
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
		) (map[string]*disc.Discovery, map[string]*peers.Manager, error) {
			archivalPeerManager, err := peers.NewManager(
				cfg.PeerManagerParams,
				h,
				gater,
				peers.WithTag(archivalNodesTag),
			)
			if err != nil {
				return nil, nil, err
			}
			lc.Append(fx.Hook{
				OnStart: archivalPeerManager.Start,
				OnStop:  archivalPeerManager.Stop,
			})

			discConfig := *cfg.Discovery
			if tp == node.Bridge || tp == node.Full && pruneCfg.EnableService {
				// TODO @renaynay: REMOVE THIS
				// TODO @renaynay: EnableAdvertise is true by default for bridges and fulls and
				//  as there is no separation for configs per discovery instance, we have to
				//  do this ugly check here to disable advertisement on the archival topic if
				//  pruning is enabled.
				discConfig.EnableAdvertise = false
			}

			archivalDisc, err := disc.NewDiscovery(
				&discConfig,
				h,
				routingdisc.NewRoutingDiscovery(r),
				archivalNodesTag,
				disc.WithOnPeersUpdate(archivalPeerManager.UpdateNodePool),
			)
			if err != nil {
				return nil, nil, err
			}

			discoveries := map[string]*disc.Discovery{fullNodesTag: d, archivalNodesTag: archivalDisc}
			managers := map[string]*peers.Manager{fullNodesTag: manager, archivalNodesTag: archivalPeerManager}

			return discoveries, managers, nil
		})
}
