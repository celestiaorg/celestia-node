package p2p

import (
	"context"
	"fmt"
	"reflect"

	libhost "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	rcmgr "github.com/libp2p/go-libp2p-resource-manager"
	basichost "github.com/libp2p/go-libp2p/p2p/host/basic"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
)

// Module represents all accessible methods related to the node's p2p
// host / operations.
//
//nolint:dupl
type Module interface {
	// Info returns address information about the host.
	Info() peer.AddrInfo
	// Peers returns all peer IDs used across all inner stores.
	Peers() []peer.ID
	// PeerInfo returns a small slice of information Peerstore has on the
	// given peer.
	PeerInfo(id peer.ID) peer.AddrInfo

	// Connect ensures there is a connection between this host and the peer with
	// given peer.
	Connect(ctx context.Context, pi peer.AddrInfo) error
	// ClosePeer closes the connection to a given peer.
	ClosePeer(id peer.ID) error
	// Connectedness returns a state signaling connection capabilities.
	Connectedness(id peer.ID) network.Connectedness
	// NATStatus returns the current NAT status.
	NATStatus() (network.Reachability, error)

	// BlockPeer adds a peer to the set of blocked peers.
	BlockPeer(p peer.ID) error
	// UnblockPeer removes a peer from the set of blocked peers.
	UnblockPeer(p peer.ID) error
	// ListBlockedPeers returns a list of blocked peers.
	ListBlockedPeers() []peer.ID
	// Protect adds a peer to the list of peers who have a bidirectional
	// peering agreement that they are protected from being trimmed, dropped
	// or negatively scored.
	Protect(id peer.ID, tag string)
	// Unprotect removes a peer from the list of peers who have a bidirectional
	// peering agreement that they are protected from being trimmed, dropped
	// or negatively scored, returning a bool representing whether the given
	// peer is protected or not.
	Unprotect(id peer.ID, tag string) bool
	// IsProtected returns whether the given peer is protected.
	IsProtected(id peer.ID, tag string) bool

	// BandwidthStats returns a Stats struct with bandwidth metrics for all
	// data sent/received by the local peer, regardless of protocol or remote
	// peer IDs.
	BandwidthStats() metrics.Stats
	// BandwidthForPeer returns a Stats struct with bandwidth metrics associated with the given peer.ID.
	// The metrics returned include all traffic sent / received for the peer, regardless of protocol.
	BandwidthForPeer(id peer.ID) metrics.Stats
	// BandwidthForProtocol returns a Stats struct with bandwidth metrics associated with the given protocol.ID.
	BandwidthForProtocol(proto protocol.ID) metrics.Stats

	// ResourceState returns the state of the resource manager.
	ResourceState() (rcmgr.ResourceManagerStat, error)

	// PubSubPeers returns the peer IDs of the peers joined on
	// the given topic.
	PubSubPeers(topic string) []peer.ID
}

// module contains all components necessary to access information and
// perform actions related to the node's p2p Host / operations.
type module struct {
	host      HostBase
	ps        *pubsub.PubSub
	connGater *conngater.BasicConnectionGater
	bw        *metrics.BandwidthCounter
	rm        network.ResourceManager
}

func newModule(
	host HostBase,
	ps *pubsub.PubSub,
	cg *conngater.BasicConnectionGater,
	bw *metrics.BandwidthCounter,
	rm network.ResourceManager,
) Module {
	return &module{
		host:      host,
		ps:        ps,
		connGater: cg,
		bw:        bw,
		rm:        rm,
	}
}

func (m *module) Info() peer.AddrInfo {
	return *libhost.InfoFromHost(m.host)
}

func (m *module) Peers() []peer.ID {
	return m.host.Peerstore().Peers()
}

func (m *module) PeerInfo(id peer.ID) peer.AddrInfo {
	return m.host.Peerstore().PeerInfo(id)
}

func (m *module) Connect(ctx context.Context, pi peer.AddrInfo) error {
	return m.host.Connect(ctx, pi)
}

func (m *module) ClosePeer(id peer.ID) error {
	return m.host.Network().ClosePeer(id)
}

func (m *module) Connectedness(id peer.ID) network.Connectedness {
	return m.host.Network().Connectedness(id)
}

func (m *module) NATStatus() (network.Reachability, error) {
	basic, ok := m.host.(*basichost.BasicHost)
	if !ok {
		return 0, fmt.Errorf("unexpected implementation of host.Host, expected %s, got %T",
			reflect.TypeOf(&basichost.BasicHost{}).String(), m.host)
	}
	return basic.GetAutoNat().Status(), nil
}

func (m *module) BlockPeer(p peer.ID) error {
	return m.connGater.BlockPeer(p)
}

func (m *module) UnblockPeer(p peer.ID) error {
	return m.connGater.UnblockPeer(p)
}

func (m *module) ListBlockedPeers() []peer.ID {
	return m.connGater.ListBlockedPeers()
}

func (m *module) Protect(id peer.ID, tag string) {
	m.host.ConnManager().Protect(id, tag)
}

func (m *module) Unprotect(id peer.ID, tag string) bool {
	return m.host.ConnManager().Unprotect(id, tag)
}

func (m *module) IsProtected(id peer.ID, tag string) bool {
	return m.host.ConnManager().IsProtected(id, tag)
}

func (m *module) BandwidthStats() metrics.Stats {
	return m.bw.GetBandwidthTotals()
}

func (m *module) BandwidthForPeer(id peer.ID) metrics.Stats {
	return m.bw.GetBandwidthForPeer(id)
}

func (m *module) BandwidthForProtocol(proto protocol.ID) metrics.Stats {
	return m.bw.GetBandwidthForProtocol(proto)
}

func (m *module) ResourceState() (rcmgr.ResourceManagerStat, error) {
	rms, ok := m.rm.(rcmgr.ResourceManagerState)
	if !ok {
		return rcmgr.ResourceManagerStat{}, fmt.Errorf("network.ResourceManager does not implement " +
			"rcmgr.ResourceManagerState")
	}
	return rms.Stat(), nil
}

func (m *module) PubSubPeers(topic string) []peer.ID {
	return m.ps.ListPeers(topic)
}

// API is a wrapper around Module for the RPC.
// TODO(@distractedm1nd): These structs need to be autogenerated.
//
//nolint:dupl
type API struct {
	Info                 func() peer.AddrInfo
	Peers                func() []peer.ID
	PeerInfo             func(id peer.ID) peer.AddrInfo
	Connect              func(ctx context.Context, pi peer.AddrInfo) error
	ClosePeer            func(id peer.ID) error
	Connectedness        func(id peer.ID) network.Connectedness
	NATStatus            func() (network.Reachability, error)
	BlockPeer            func(p peer.ID) error
	UnblockPeer          func(p peer.ID) error
	ListBlockedPeers     func() []peer.ID
	Protect              func(id peer.ID, tag string)
	Unprotect            func(id peer.ID, tag string) bool
	IsProtected          func(id peer.ID, tag string) bool
	BandwidthStats       func() metrics.Stats
	BandwidthForPeer     func(id peer.ID) metrics.Stats
	BandwidthForProtocol func(proto protocol.ID) metrics.Stats
	ResourceState        func() (rcmgr.ResourceManagerStat, error)
	PubSubPeers          func(topic string) []peer.ID
}
