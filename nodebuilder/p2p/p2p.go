package p2p

import (
	"context"
	"fmt"
	"reflect"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	libhost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/host/autonat"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
)

var _ Module = (*API)(nil)

// ConnectionState holds information about a connection.
type ConnectionState struct {
	Info network.ConnectionState
	// NumStreams is the number of streams on the connection.
	NumStreams int
	// Direction specifies whether this is an inbound or an outbound connection.
	Direction network.Direction
	// Opened is the timestamp when this connection was opened.
	Opened time.Time
	// Limited indicates that this connection is Limited. It maybe limited by
	// bytes or time. In practice, this is a connection formed over a circuit v2
	// relay.
	Limited bool
}

// Module represents all accessible methods related to the node's p2p
// host / operations.
//
//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
type Module interface {
	// Info returns address information about the host.
	Info(context.Context) (peer.AddrInfo, error)
	// Network returns the host's network/chain ID.
	Network(context.Context) (string, error)
	// Peers returns connected peers.
	Peers(context.Context) ([]peer.ID, error)
	// PeerInfo returns a small slice of information Peerstore has on the
	// given peer.
	PeerInfo(ctx context.Context, id peer.ID) (peer.AddrInfo, error)

	// Connect ensures there is a connection between this host and the peer with
	// given peer.
	Connect(ctx context.Context, pi peer.AddrInfo) error
	// ClosePeer closes the connection to a given peer.
	ClosePeer(ctx context.Context, id peer.ID) error
	// Connectedness returns a state signaling connection capabilities.
	Connectedness(ctx context.Context, id peer.ID) (network.Connectedness, error)
	// ConnectionState returns information about each *active* connection to the peer.
	// NOTE: At most cases there should be only a single connection.
	ConnectionState(ctx context.Context, id peer.ID) ([]ConnectionState, error)
	// NATStatus returns the current NAT status.
	NATStatus(context.Context) (network.Reachability, error)

	// BlockPeer adds a peer to the set of blocked peers and
	// closes any existing connection to that peer.
	BlockPeer(ctx context.Context, p peer.ID) error
	// UnblockPeer removes a peer from the set of blocked peers.
	UnblockPeer(ctx context.Context, p peer.ID) error
	// ListBlockedPeers returns a list of blocked peers.
	ListBlockedPeers(context.Context) ([]peer.ID, error)
	// Protect adds a peer to the list of peers who have a bidirectional
	// peering agreement that they are protected from being trimmed, dropped
	// or negatively scored.
	Protect(ctx context.Context, id peer.ID, tag string) error
	// Unprotect removes a peer from the list of peers who have a bidirectional
	// peering agreement that they are protected from being trimmed, dropped
	// or negatively scored, returning a bool representing whether the given
	// peer is protected or not.
	Unprotect(ctx context.Context, id peer.ID, tag string) (bool, error)
	// IsProtected returns whether the given peer is protected.
	IsProtected(ctx context.Context, id peer.ID, tag string) (bool, error)

	// BandwidthStats returns a Stats struct with bandwidth metrics for all
	// data sent/received by the local peer, regardless of protocol or remote
	// peer IDs.
	BandwidthStats(context.Context) (metrics.Stats, error)
	// BandwidthForPeer returns a Stats struct with bandwidth metrics associated with the given peer.ID.
	// The metrics returned include all traffic sent / received for the peer, regardless of protocol.
	BandwidthForPeer(ctx context.Context, id peer.ID) (metrics.Stats, error)
	// BandwidthForProtocol returns a Stats struct with bandwidth metrics associated with the given
	// protocol.ID.
	BandwidthForProtocol(ctx context.Context, proto protocol.ID) (metrics.Stats, error)

	// ResourceState returns the state of the resource manager.
	ResourceState(context.Context) (rcmgr.ResourceManagerStat, error)

	// PubSubPeers returns the peer IDs of the peers joined on
	// the given topic.
	PubSubPeers(ctx context.Context, topic string) ([]peer.ID, error)
	// PubSubTopics reports current PubSubTopics the node participates in.
	PubSubTopics(ctx context.Context) ([]string, error)

	// Ping pings the selected peer and returns time it took or error.
	Ping(ctx context.Context, peer peer.ID) (time.Duration, error)
}

// module contains all components necessary to access information and
// perform actions related to the node's p2p Host / operations.
type module struct {
	host      HostBase
	ps        *pubsub.PubSub
	connGater *conngater.BasicConnectionGater
	bw        *metrics.BandwidthCounter
	rm        network.ResourceManager
	networkID Network
}

func newModule(
	host HostBase,
	ps *pubsub.PubSub,
	cg *conngater.BasicConnectionGater,
	bw *metrics.BandwidthCounter,
	rm network.ResourceManager,
	network Network,
) Module {
	return &module{
		host:      host,
		ps:        ps,
		connGater: cg,
		bw:        bw,
		rm:        rm,
		networkID: network,
	}
}

func (m *module) Info(context.Context) (peer.AddrInfo, error) {
	return *libhost.InfoFromHost(m.host), nil
}

func (m *module) Network(context.Context) (string, error) {
	return m.networkID.String(), nil
}

func (m *module) Peers(context.Context) ([]peer.ID, error) {
	return m.host.Network().Peers(), nil
}

func (m *module) PeerInfo(_ context.Context, id peer.ID) (peer.AddrInfo, error) {
	return m.host.Peerstore().PeerInfo(id), nil
}

func (m *module) Connect(ctx context.Context, pi peer.AddrInfo) error {
	return m.host.Connect(ctx, pi)
}

func (m *module) ClosePeer(_ context.Context, id peer.ID) error {
	return m.host.Network().ClosePeer(id)
}

func (m *module) Connectedness(_ context.Context, id peer.ID) (network.Connectedness, error) {
	return m.host.Network().Connectedness(id), nil
}

type autoNatGetter interface {
	GetAutoNat() autonat.AutoNAT
}

func (m *module) NATStatus(context.Context) (network.Reachability, error) {
	basic, ok := m.host.(autoNatGetter)
	if !ok {
		return 0, fmt.Errorf("unexpected implementation of host.Host, expected %s, got %T",
			reflect.TypeOf((*autoNatGetter)(nil)).String(), m.host)
	}
	return basic.GetAutoNat().Status(), nil
}

func (m *module) BlockPeer(_ context.Context, p peer.ID) error {
	if err := m.connGater.BlockPeer(p); err != nil {
		return err
	}
	if err := m.host.Network().ClosePeer(p); err != nil {
		log.Warnf("failed to close connection to blocked peer %s: %v", p, err)
	}
	return nil
}

func (m *module) UnblockPeer(_ context.Context, p peer.ID) error {
	return m.connGater.UnblockPeer(p)
}

func (m *module) ListBlockedPeers(context.Context) ([]peer.ID, error) {
	return m.connGater.ListBlockedPeers(), nil
}

func (m *module) Protect(_ context.Context, id peer.ID, tag string) error {
	m.host.ConnManager().Protect(id, tag)
	return nil
}

func (m *module) Unprotect(_ context.Context, id peer.ID, tag string) (bool, error) {
	return m.host.ConnManager().Unprotect(id, tag), nil
}

func (m *module) IsProtected(_ context.Context, id peer.ID, tag string) (bool, error) {
	return m.host.ConnManager().IsProtected(id, tag), nil
}

func (m *module) BandwidthStats(context.Context) (metrics.Stats, error) {
	return m.bw.GetBandwidthTotals(), nil
}

func (m *module) BandwidthForPeer(_ context.Context, id peer.ID) (metrics.Stats, error) {
	return m.bw.GetBandwidthForPeer(id), nil
}

func (m *module) BandwidthForProtocol(_ context.Context, proto protocol.ID) (metrics.Stats, error) {
	return m.bw.GetBandwidthForProtocol(proto), nil
}

func (m *module) ResourceState(context.Context) (rcmgr.ResourceManagerStat, error) {
	rms, ok := m.rm.(rcmgr.ResourceManagerState)
	if !ok {
		return rcmgr.ResourceManagerStat{}, fmt.Errorf("network.resourceManager does not implement " +
			"rcmgr.ResourceManagerState")
	}
	return rms.Stat(), nil
}

func (m *module) PubSubPeers(_ context.Context, topic string) ([]peer.ID, error) {
	return m.ps.ListPeers(topic), nil
}

func (m *module) PubSubTopics(_ context.Context) ([]string, error) {
	return m.ps.GetTopics(), nil
}

func (m *module) Ping(ctx context.Context, peer peer.ID) (time.Duration, error) {
	res := <-ping.Ping(ctx, m.host, peer) // context is handled for us
	return res.RTT, res.Error
}

func (m *module) ConnectionState(_ context.Context, peer peer.ID) ([]ConnectionState, error) {
	cons := m.host.Network().ConnsToPeer(peer)
	if len(cons) == 0 {
		return nil, fmt.Errorf("no connections to peer %s", peer)
	}

	conInfos := make([]ConnectionState, len(cons))
	for i, con := range cons {
		stat := con.Stat()
		conInfos[i] = ConnectionState{
			Info:       con.ConnState(),
			NumStreams: stat.NumStreams,
			Direction:  stat.Direction,
			Opened:     stat.Opened,
			Limited:    stat.Limited,
		}
	}

	return conInfos, nil
}

// API is a wrapper around Module for the RPC.
type API struct {
	Internal struct {
		Info                 func(context.Context) (peer.AddrInfo, error)                         `perm:"admin"`
		Network              func(context.Context) (string, error)                                `perm:"admin"`
		Peers                func(context.Context) ([]peer.ID, error)                             `perm:"admin"`
		PeerInfo             func(ctx context.Context, id peer.ID) (peer.AddrInfo, error)         `perm:"admin"`
		Connect              func(ctx context.Context, pi peer.AddrInfo) error                    `perm:"admin"`
		ClosePeer            func(ctx context.Context, id peer.ID) error                          `perm:"admin"`
		Connectedness        func(ctx context.Context, id peer.ID) (network.Connectedness, error) `perm:"admin"`
		NATStatus            func(context.Context) (network.Reachability, error)                  `perm:"admin"`
		BlockPeer            func(ctx context.Context, p peer.ID) error                           `perm:"admin"`
		UnblockPeer          func(ctx context.Context, p peer.ID) error                           `perm:"admin"`
		ListBlockedPeers     func(context.Context) ([]peer.ID, error)                             `perm:"admin"`
		Protect              func(ctx context.Context, id peer.ID, tag string) error              `perm:"admin"`
		Unprotect            func(ctx context.Context, id peer.ID, tag string) (bool, error)      `perm:"admin"`
		IsProtected          func(ctx context.Context, id peer.ID, tag string) (bool, error)      `perm:"admin"`
		BandwidthStats       func(context.Context) (metrics.Stats, error)                         `perm:"admin"`
		BandwidthForPeer     func(ctx context.Context, id peer.ID) (metrics.Stats, error)         `perm:"admin"`
		BandwidthForProtocol func(ctx context.Context, proto protocol.ID) (metrics.Stats, error)  `perm:"admin"`
		ResourceState        func(context.Context) (rcmgr.ResourceManagerStat, error)             `perm:"admin"`
		PubSubPeers          func(ctx context.Context, topic string) ([]peer.ID, error)           `perm:"admin"`
		PubSubTopics         func(ctx context.Context) ([]string, error)                          `perm:"admin"`
		Ping                 func(ctx context.Context, peer peer.ID) (time.Duration, error)       `perm:"admin"`
		ConnectionState      func(context.Context, peer.ID) ([]ConnectionState, error)            `perm:"admin"`
	}
}

func (api *API) Info(ctx context.Context) (peer.AddrInfo, error) {
	return api.Internal.Info(ctx)
}

func (api *API) Network(ctx context.Context) (string, error) {
	return api.Internal.Network(ctx)
}

func (api *API) Peers(ctx context.Context) ([]peer.ID, error) {
	return api.Internal.Peers(ctx)
}

func (api *API) PeerInfo(ctx context.Context, id peer.ID) (peer.AddrInfo, error) {
	return api.Internal.PeerInfo(ctx, id)
}

func (api *API) Connect(ctx context.Context, pi peer.AddrInfo) error {
	return api.Internal.Connect(ctx, pi)
}

func (api *API) ClosePeer(ctx context.Context, id peer.ID) error {
	return api.Internal.ClosePeer(ctx, id)
}

func (api *API) Connectedness(ctx context.Context, id peer.ID) (network.Connectedness, error) {
	return api.Internal.Connectedness(ctx, id)
}

func (api *API) NATStatus(ctx context.Context) (network.Reachability, error) {
	return api.Internal.NATStatus(ctx)
}

func (api *API) BlockPeer(ctx context.Context, p peer.ID) error {
	return api.Internal.BlockPeer(ctx, p)
}

func (api *API) UnblockPeer(ctx context.Context, p peer.ID) error {
	return api.Internal.UnblockPeer(ctx, p)
}

func (api *API) ListBlockedPeers(ctx context.Context) ([]peer.ID, error) {
	return api.Internal.ListBlockedPeers(ctx)
}

func (api *API) Protect(ctx context.Context, id peer.ID, tag string) error {
	return api.Internal.Protect(ctx, id, tag)
}

func (api *API) Unprotect(ctx context.Context, id peer.ID, tag string) (bool, error) {
	return api.Internal.Unprotect(ctx, id, tag)
}

func (api *API) IsProtected(ctx context.Context, id peer.ID, tag string) (bool, error) {
	return api.Internal.IsProtected(ctx, id, tag)
}

func (api *API) BandwidthStats(ctx context.Context) (metrics.Stats, error) {
	return api.Internal.BandwidthStats(ctx)
}

func (api *API) BandwidthForPeer(ctx context.Context, id peer.ID) (metrics.Stats, error) {
	return api.Internal.BandwidthForPeer(ctx, id)
}

func (api *API) BandwidthForProtocol(ctx context.Context, proto protocol.ID) (metrics.Stats, error) {
	return api.Internal.BandwidthForProtocol(ctx, proto)
}

func (api *API) ResourceState(ctx context.Context) (rcmgr.ResourceManagerStat, error) {
	return api.Internal.ResourceState(ctx)
}

func (api *API) PubSubPeers(ctx context.Context, topic string) ([]peer.ID, error) {
	return api.Internal.PubSubPeers(ctx, topic)
}

func (api *API) PubSubTopics(ctx context.Context) ([]string, error) {
	return api.Internal.PubSubTopics(ctx)
}

func (api *API) Ping(ctx context.Context, peer peer.ID) (time.Duration, error) {
	return api.Internal.Ping(ctx, peer)
}

func (api *API) ConnectionState(ctx context.Context, peer peer.ID) ([]ConnectionState, error) {
	return api.Internal.ConnectionState(ctx, peer)
}
