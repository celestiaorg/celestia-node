package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/exchange"
	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	hst "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

var log = logging.Logger("module/p2p")

// newModuleDisabled creates a disabled P2P module that returns errors for all operations.
func newModuleDisabled() Module {
	return &moduleDisabled{}
}

// moduleDisabled is a stub implementation of Module for when P2P is disabled.
type moduleDisabled struct{}

func (m *moduleDisabled) Info(context.Context) (peer.AddrInfo, error) {
	return peer.AddrInfo{}, fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) Peers(context.Context) ([]peer.ID, error) {
	return nil, fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) PeerInfo(context.Context, peer.ID) (peer.AddrInfo, error) {
	return peer.AddrInfo{}, fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) Connect(context.Context, peer.AddrInfo) error {
	return fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) ClosePeer(context.Context, peer.ID) error {
	return fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) Connectedness(context.Context, peer.ID) (network.Connectedness, error) {
	return network.NotConnected, fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) NATStatus(context.Context) (network.Reachability, error) {
	return network.ReachabilityUnknown, fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) BlockPeer(context.Context, peer.ID) error {
	return fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) UnblockPeer(context.Context, peer.ID) error {
	return fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) ListBlockedPeers(context.Context) ([]peer.ID, error) {
	return nil, nil
}

func (m *moduleDisabled) Protect(context.Context, peer.ID, string) error {
	return fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) Unprotect(context.Context, peer.ID, string) (bool, error) {
	return false, fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) IsProtected(context.Context, peer.ID, string) (bool, error) {
	return false, fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) BandwidthStats(context.Context) (metrics.Stats, error) {
	return metrics.Stats{}, nil
}

func (m *moduleDisabled) BandwidthForPeer(context.Context, peer.ID) (metrics.Stats, error) {
	return metrics.Stats{}, fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) BandwidthForProtocol(context.Context, protocol.ID) (metrics.Stats, error) {
	return metrics.Stats{}, fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) ResourceState(context.Context) (rcmgr.ResourceManagerStat, error) {
	return rcmgr.ResourceManagerStat{}, nil
}

func (m *moduleDisabled) PubSubPeers(context.Context, string) ([]peer.ID, error) {
	return nil, fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) PubSubTopics(context.Context) ([]string, error) {
	return nil, nil
}

func (m *moduleDisabled) Ping(context.Context, peer.ID) (time.Duration, error) {
	return 0, fmt.Errorf("p2p is disabled")
}

func (m *moduleDisabled) ConnectionState(context.Context, peer.ID) ([]ConnectionState, error) {
	return nil, fmt.Errorf("p2p is disabled")
}

// ConstructModule collects all the components and services related to p2p.
func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	// If P2P is disabled, return minimal module with nil/empty implementations
	if cfg.Disabled {
		return fx.Module(
			"p2p",
			fx.Supply(cfg),
			// Provide nil implementations for optional dependencies
			fx.Provide(func() HostBase { return nil }),
			fx.Provide(func() hst.Host { return nil }),
			fx.Provide(func() *pubsub.PubSub { return nil }),
			fx.Provide(func() routing.PeerRouting { return nil }),
			fx.Provide(func() exchange.SessionExchange { return nil }),
			fx.Provide(func() blockservice.BlockService { return nil }),
			fx.Provide(func() *conngater.BasicConnectionGater { return nil }),
			fx.Provide(func() *metrics.BandwidthCounter { return nil }),
			fx.Provide(func() network.ResourceManager { return nil }),
			fx.Provide(newModuleDisabled),
		)
	}

	// sanitize config values before constructing module
	baseComponents := fx.Options(
		fx.Supply(cfg),
		fx.Provide(Key),
		fx.Provide(id),
		fx.Provide(peerStore),
		fx.Provide(connectionManager),
		fx.Provide(connectionGater),
		fx.Provide(newHost),
		fx.Provide(routedHost),
		fx.Provide(pubSub),
		fx.Provide(ipld.NewBlockservice),
		fx.Provide(peerRouting),
		fx.Provide(newDHT),
		fx.Provide(addrsFactory(cfg.AnnounceAddresses, cfg.NoAnnounceAddresses)),
		fx.Provide(metrics.NewBandwidthCounter),
		fx.Provide(newModule),
		fx.Invoke(Listen(cfg)),
		fx.Provide(resourceManager),
		fx.Provide(resourceManagerOpt(allowList)),
	)

	switch tp {
	case node.Full, node.Bridge:
		return fx.Module(
			"p2p",
			baseComponents,
			fx.Provide(infiniteResources),
			fx.Invoke(reachabilityCheck),
			fx.Invoke(connectToBootstrappers),
		)
	case node.Light:
		return fx.Module(
			"p2p",
			baseComponents,
			fx.Provide(autoscaleResources),
		)
	default:
		panic("invalid node type")
	}
}
