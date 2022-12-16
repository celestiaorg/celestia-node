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
	basichost "github.com/libp2p/go-libp2p/p2p/host/basic"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
)

var _ Module = (*API)(nil)

// Module represents all accessible methods related to the node's p2p
// host / operations.
//
//nolint:dupl
//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
type Module interface {
	// Info returns address information about the host.
	Info(context.Context) peer.AddrInfo
	// Peers returns all peer IDs used across all inner stores.
	Peers(context.Context) []peer.ID
	// PeerInfo returns a small slice of information Peerstore has on the
	// given peer.
	PeerInfo(ctx context.Context, id peer.ID) peer.AddrInfo

	// Connect ensures there is a connection between this host and the peer with
	// given peer.
	Connect(ctx context.Context, pi peer.AddrInfo) error
	// ClosePeer closes the connection to a given peer.
	ClosePeer(ctx context.Context, id peer.ID) error
	// Connectedness returns a state signaling connection capabilities.
	Connectedness(ctx context.Context, id peer.ID) network.Connectedness
	// NATStatus returns the current NAT status.
	NATStatus(context.Context) (network.Reachability, error)

	// BlockPeer adds a peer to the set of blocked peers.
	BlockPeer(ctx context.Context, p peer.ID) error
	// UnblockPeer removes a peer from the set of blocked peers.
	UnblockPeer(ctx context.Context, p peer.ID) error
	// ListBlockedPeers returns a list of blocked peers.
	ListBlockedPeers(context.Context) []peer.ID
	// Protect adds a peer to the list of peers who have a bidirectional
	// peering agreement that they are protected from being trimmed, dropped
	// or negatively scored.
	Protect(ctx context.Context, id peer.ID, tag string)
	// Unprotect removes a peer from the list of peers who have a bidirectional
	// peering agreement that they are protected from being trimmed, dropped
	// or negatively scored, returning a bool representing whether the given
	// peer is protected or not.
	Unprotect(ctx context.Context, id peer.ID, tag string) bool
	// IsProtected returns whether the given peer is protected.
	IsProtected(ctx context.Context, id peer.ID, tag string) bool

	// BandwidthStats returns a Stats struct with bandwidth metrics for all
	// data sent/received by the local peer, regardless of protocol or remote
	// peer IDs.
	BandwidthStats(context.Context) metrics.Stats
	// BandwidthForPeer returns a Stats struct with bandwidth metrics associated with the given peer.ID.
	// The metrics returned include all traffic sent / received for the peer, regardless of protocol.
	BandwidthForPeer(ctx context.Context, id peer.ID) metrics.Stats
	// BandwidthForProtocol returns a Stats struct with bandwidth metrics associated with the given protocol.ID.
	BandwidthForProtocol(ctx context.Context, proto protocol.ID) metrics.Stats

	// PubSubPeers returns the peer IDs of the peers joined on
	// the given topic.
	PubSubPeers(ctx context.Context, topic string) []peer.ID
}

// module contains all components necessary to access information and
// perform actions related to the node's p2p Host / operations.
type module struct {
	host      HostBase
	ps        *pubsub.PubSub
	connGater *conngater.BasicConnectionGater
	bw        *metrics.BandwidthCounter
}

func newModule(
	host HostBase,
	ps *pubsub.PubSub,
	cg *conngater.BasicConnectionGater,
	bw *metrics.BandwidthCounter,
) Module {
	return &module{
		host:      host,
		ps:        ps,
		connGater: cg,
		bw:        bw,
	}
}

func (m *module) Info(context.Context) peer.AddrInfo {
	return *libhost.InfoFromHost(m.host)
}

func (m *module) Peers(context.Context) []peer.ID {
	return m.host.Peerstore().Peers()
}

func (m *module) PeerInfo(_ context.Context, id peer.ID) peer.AddrInfo {
	return m.host.Peerstore().PeerInfo(id)
}

func (m *module) Connect(ctx context.Context, pi peer.AddrInfo) error {
	return m.host.Connect(ctx, pi)
}

func (m *module) ClosePeer(_ context.Context, id peer.ID) error {
	return m.host.Network().ClosePeer(id)
}

func (m *module) Connectedness(_ context.Context, id peer.ID) network.Connectedness {
	return m.host.Network().Connectedness(id)
}

func (m *module) NATStatus(context.Context) (network.Reachability, error) {
	basic, ok := m.host.(*basichost.BasicHost)
	if !ok {
		return 0, fmt.Errorf("unexpected implementation of host.Host, expected %s, got %T",
			reflect.TypeOf(&basichost.BasicHost{}).String(), m.host)
	}
	return basic.GetAutoNat().Status(), nil
}

func (m *module) BlockPeer(_ context.Context, p peer.ID) error {
	return m.connGater.BlockPeer(p)
}

func (m *module) UnblockPeer(_ context.Context, p peer.ID) error {
	return m.connGater.UnblockPeer(p)
}

func (m *module) ListBlockedPeers(context.Context) []peer.ID {
	return m.connGater.ListBlockedPeers()
}

func (m *module) Protect(_ context.Context, id peer.ID, tag string) {
	m.host.ConnManager().Protect(id, tag)
}

func (m *module) Unprotect(_ context.Context, id peer.ID, tag string) bool {
	return m.host.ConnManager().Unprotect(id, tag)
}

func (m *module) IsProtected(_ context.Context, id peer.ID, tag string) bool {
	return m.host.ConnManager().IsProtected(id, tag)
}

func (m *module) BandwidthStats(context.Context) metrics.Stats {
	return m.bw.GetBandwidthTotals()
}

func (m *module) BandwidthForPeer(_ context.Context, id peer.ID) metrics.Stats {
	return m.bw.GetBandwidthForPeer(id)
}

func (m *module) BandwidthForProtocol(_ context.Context, proto protocol.ID) metrics.Stats {
	return m.bw.GetBandwidthForProtocol(proto)
}

func (m *module) PubSubPeers(_ context.Context, topic string) []peer.ID {
	return m.ps.ListPeers(topic)
}

// API is a wrapper around Module for the RPC.
// TODO(@distractedm1nd): These structs need to be autogenerated.
//
//nolint:dupl
type API struct {
	Internal struct {
		Info                 func(context.Context) peer.AddrInfo                         `perm:"admin"`
		Peers                func(context.Context) []peer.ID                             `perm:"admin"`
		PeerInfo             func(ctx context.Context, id peer.ID) peer.AddrInfo         `perm:"admin"`
		Connect              func(ctx context.Context, pi peer.AddrInfo) error           `perm:"admin"`
		ClosePeer            func(ctx context.Context, id peer.ID) error                 `perm:"admin"`
		Connectedness        func(ctx context.Context, id peer.ID) network.Connectedness `perm:"admin"`
		NATStatus            func(context.Context) (network.Reachability, error)         `perm:"admin"`
		BlockPeer            func(ctx context.Context, p peer.ID) error                  `perm:"admin"`
		UnblockPeer          func(ctx context.Context, p peer.ID) error                  `perm:"admin"`
		ListBlockedPeers     func(context.Context) []peer.ID                             `perm:"admin"`
		Protect              func(ctx context.Context, id peer.ID, tag string)           `perm:"admin"`
		Unprotect            func(ctx context.Context, id peer.ID, tag string) bool      `perm:"admin"`
		IsProtected          func(ctx context.Context, id peer.ID, tag string) bool      `perm:"admin"`
		BandwidthStats       func(context.Context) metrics.Stats                         `perm:"admin"`
		BandwidthForPeer     func(ctx context.Context, id peer.ID) metrics.Stats         `perm:"admin"`
		BandwidthForProtocol func(ctx context.Context, proto protocol.ID) metrics.Stats  `perm:"admin"`
		PubSubPeers          func(ctx context.Context, topic string) []peer.ID           `perm:"admin"`
	}
}

func (api *API) Info(ctx context.Context) peer.AddrInfo {
	return api.Internal.Info(ctx)
}

func (api *API) Peers(ctx context.Context) []peer.ID {
	return api.Internal.Peers(ctx)
}

func (api *API) PeerInfo(ctx context.Context, id peer.ID) peer.AddrInfo {
	return api.Internal.PeerInfo(ctx, id)
}

func (api *API) Connect(ctx context.Context, pi peer.AddrInfo) error {
	return api.Internal.Connect(ctx, pi)
}

func (api *API) ClosePeer(ctx context.Context, id peer.ID) error {
	return api.Internal.ClosePeer(ctx, id)
}

func (api *API) Connectedness(ctx context.Context, id peer.ID) network.Connectedness {
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

func (api *API) ListBlockedPeers(ctx context.Context) []peer.ID {
	return api.Internal.ListBlockedPeers(ctx)
}

func (api *API) Protect(ctx context.Context, id peer.ID, tag string) {
	api.Internal.Protect(ctx, id, tag)
}

func (api *API) Unprotect(ctx context.Context, id peer.ID, tag string) bool {
	return api.Internal.Unprotect(ctx, id, tag)
}

func (api *API) IsProtected(ctx context.Context, id peer.ID, tag string) bool {
	return api.Internal.IsProtected(ctx, id, tag)
}

func (api *API) BandwidthStats(ctx context.Context) metrics.Stats {
	return api.Internal.BandwidthStats(ctx)
}

func (api *API) BandwidthForPeer(ctx context.Context, id peer.ID) metrics.Stats {
	return api.Internal.BandwidthForPeer(ctx, id)
}

func (api *API) BandwidthForProtocol(ctx context.Context, proto protocol.ID) metrics.Stats {
	return api.Internal.BandwidthForProtocol(ctx, proto)
}

func (api *API) PubSubPeers(ctx context.Context, topic string) []peer.ID {
	return api.Internal.PubSubPeers(ctx, topic)
}
