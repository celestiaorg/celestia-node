package p2p

import (
	"context"
	"fmt"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	rcmgr "github.com/libp2p/go-libp2p-resource-manager"
	"github.com/libp2p/go-libp2p/p2p/host/autonat"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
)

type API struct {
	Info                 func() Info
	Peers                func() peer.IDSlice
	PeerInfo             func(id peer.ID) peer.AddrInfo
	Connect              func(ctx context.Context, pi peer.AddrInfo) error
	ClosePeer            func(id peer.ID) error
	Connectedness        func(id peer.ID) network.Connectedness
	NATStatus            func() network.Reachability
	BlockPeer            func(p peer.ID) error
	UnblockPeer          func(p peer.ID) error
	ListBlockedPeers     func() []peer.ID
	MutualAdd            func(id peer.ID, tag string)
	MutualRm             func(id peer.ID, tag string) bool
	IsMutual             func(id peer.ID, tag string) bool
	BandwidthStats       func() metrics.Stats
	BandwidthForPeer     func(id peer.ID) metrics.Stats
	BandwidthForProtocol func(proto protocol.ID) metrics.Stats
	ResourceState        func(rcmgr.ResourceManagerStat, error)
	PubSubPeers          func(topic string) []peer.ID
}

// Module represents all accessible methods related to the node's p2p
// host / operations.
type Module interface {
	// Info returns basic information about the node's p2p host/operations.
	Info() Info
	// Peers returns all peer IDs used across all inner stores.
	Peers() peer.IDSlice
	// PeerInfo returns a small slice of information Peerstore has on the
	// given peer.
	PeerInfo(id peer.ID) peer.AddrInfo

	// Connect ensures there is a connection between this host and the peer with
	// given peer.ID.
	Connect(ctx context.Context, pi peer.AddrInfo) error
	// ClosePeer closes the connection to a given peer.
	ClosePeer(id peer.ID) error
	// Connectedness returns a state signaling connection capabilities.
	Connectedness(id peer.ID) network.Connectedness
	// NATStatus returns the current NAT status.
	NATStatus() network.Reachability

	// BlockPeer adds a peer to the set of blocked peers. // TODO should we wrap BlockPeer so
	//  that it 1. disconnects from peer, then 2. adds to blocklist? cc @Wondertan
	BlockPeer(p peer.ID) error
	// UnblockPeer removes a peer from the set of blocked peers.
	UnblockPeer(p peer.ID) error
	// ListBlockedPeers returns a list of blocked peers.
	ListBlockedPeers() []peer.ID
	// MutualAdd adds a peer to the list of peers who have a bidirectional
	// peering agreement that they are protected from being trimmed, dropped
	// or negatively scored.
	MutualAdd(id peer.ID, tag string)
	// MutualRm removes a peer from the list of peers who have a bidirectional
	// peering agreement that they are protected from being trimmed, dropped
	// or negatively scored, returning a bool representing whether the given
	// peer is protected or not.
	MutualRm(id peer.ID, tag string) bool
	// IsMutual returns whether the given peer is a mutual peer.
	IsMutual(id peer.ID, tag string) bool

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

// manager contains all components necessary to access information and
// perform actions related to the node's p2p Host / operations.
type manager struct { // TODO @renaynay: rename ?
	host      HostBase
	ps        *pubsub.PubSub
	nat       autonat.AutoNAT
	connGater *conngater.BasicConnectionGater
	bw        *metrics.BandwidthCounter
	rm        network.ResourceManager
}

func newManager(
	host host.Host,
	ps *pubsub.PubSub,
	nat autonat.AutoNAT,
	cg *conngater.BasicConnectionGater,
	bw *metrics.BandwidthCounter,
	rm network.ResourceManager,
) Module {
	return &manager{
		host:      host,
		ps:        ps,
		nat:       nat,
		connGater: cg,
		bw:        bw,
		rm:        rm,
	}
}

// Info contains basic information about the node's p2p host/operations.
type Info struct {
	// ID is the node's peer ID
	ID peer.ID `json:"id"`
	// Addrs is the node's
	Addrs []ma.Multiaddr `json:"addrs"`
}

func (m *manager) Info() Info {
	m.host.Network().
	return Info{
		ID:    m.host.ID(),
		Addrs: m.host.Addrs(),
	}
}

func (m *manager) Peers() peer.IDSlice {
	return m.host.Peerstore().Peers()
}

func (m *manager) PeerInfo(id peer.ID) peer.AddrInfo {
	return m.host.Peerstore().PeerInfo(id)
}

func (m *manager) Connect(ctx context.Context, pi peer.AddrInfo) error {
	return m.host.Connect(ctx, pi)
}

func (m *manager) ClosePeer(id peer.ID) error {
	return m.host.Network().ClosePeer(id)
}

func (m *manager) Connectedness(id peer.ID) network.Connectedness {
	return m.host.Network().Connectedness(id)
}

func (m *manager) NATStatus() network.Reachability {
	return m.nat.Status()
}

func (m *manager) BlockPeer(p peer.ID) error {
	return m.connGater.BlockPeer(p)
}

func (m *manager) UnblockPeer(p peer.ID) error {
	return m.connGater.UnblockPeer(p)
}

func (m *manager) ListBlockedPeers() []peer.ID {
	return m.connGater.ListBlockedPeers()
}

func (m *manager) MutualAdd(id peer.ID, tag string) {
	m.host.ConnManager().Protect(id, tag)
}

func (m *manager) MutualRm(id peer.ID, tag string) bool {
	return m.host.ConnManager().Unprotect(id, tag)
}

func (m *manager) IsMutual(id peer.ID, tag string) bool {
	return m.host.ConnManager().IsProtected(id, tag)
}

func (m *manager) BandwidthStats() metrics.Stats {
	return m.bw.GetBandwidthTotals()
}

func (m *manager) BandwidthForPeer(id peer.ID) metrics.Stats {
	return m.bw.GetBandwidthForPeer(id)
}

func (m *manager) BandwidthForProtocol(proto protocol.ID) metrics.Stats {
	return m.bw.GetBandwidthForProtocol(proto)
}

func (m *manager) ResourceState() (rcmgr.ResourceManagerStat, error) {
	rms, ok := m.rm.(rcmgr.ResourceManagerState)
	if !ok {
		return rcmgr.ResourceManagerStat{}, fmt.Errorf("network.ResourceManager does not implement " +
			"rcmgr.ResourceManagerState") // TODO err msg?
	}
	return rms.Stat(), nil
}

func (m *manager) PubSubPeers(topic string) []peer.ID {
	return m.ps.ListPeers(topic)
}
