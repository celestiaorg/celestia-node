package share

import (
	"context"
	"time"

	core "github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/backoff"
)

const (
	// peersLimit is max amount of peers that will be discovered.
	peersLimit = 3

	// peerWeight is a weight of discovered peers.
	// peerWeight is a number that will be assigned to all discovered full nodes,
	// so ConnManager will not break a connection with them.
	peerWeight                = 1000
	topic                     = "full"
	interval                  = time.Second * 10
	defaultConnectionInterval = interval * 3
)

// waitF calculates time to restart announcing.
var waitF = func(ttl time.Duration) time.Duration {
	return 7 * ttl / 8
}

// discovery combines advertise and discover services and allows to store discovered nodes.
type discovery struct {
	set       *limitedSet
	host      host.Host
	disc      core.Discovery
	connector *backoffConnector
}

// newDiscovery constructs a new discovery.
func newDiscovery(h host.Host, d core.Discovery) *discovery {
	return &discovery{
		newLimitedSet(peersLimit),
		h,
		d,
		newBackoffConnector(h, backoff.NewFixedBackoff(time.Hour)),
	}
}

// handlePeersFound receives peers and tries to establish a connection with them.
// Peer will be added to PeerCache if connection succeeds.
func (d *discovery) handlePeerFound(ctx context.Context, topic string, peer peer.AddrInfo) {
	if peer.ID == d.host.ID() || len(peer.Addrs) == 0 || d.set.Contains(peer.ID) {
		return
	}
	err := d.set.TryAdd(peer.ID)
	if err != nil {
		log.Debug(err)
		return
	}

	err = d.connector.Connect(ctx, peer)
	if err != nil {
		log.Warn(err)
		d.set.Remove(peer.ID)
		return
	}
	log.Debugw("added peer to set", "id", peer.ID)
	// add tag to protect peer of being killed by ConnManager
	d.host.ConnManager().TagPeer(peer.ID, topic, peerWeight)
}

// ensurePeers ensures we always have 'peerLimit' connected peers.
// It starts peer discovery every 30 seconds until peer cache reaches peersLimit.
// Discovery is restarted if any previously connected peers disconnect.
func (d *discovery) ensurePeers(ctx context.Context) {
	// subscribe on Event Bus in order to catch disconnected peers and restart the discovery
	sub, err := d.host.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{})
	if err != nil {
		log.Error(err)
		return
	}
	t := time.NewTicker(defaultConnectionInterval)
	defer func() {
		t.Stop()
		if err = sub.Close(); err != nil {
			log.Error(err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if d.set.Size() >= peersLimit {
				// stop ticker if we have reached the limit
				t.Stop()
				continue
			}
			peers, err := d.disc.FindPeers(ctx, topic)
			if err != nil {
				log.Error(err)
				continue
			}
			for peer := range peers {
				go d.handlePeerFound(ctx, topic, peer)
			}
		case e := <-sub.Out():
			// listen to disconnect event to remove peer from set and reset backoff time
			// reset timer in order to restart the discovery, once stored peer is disconnected
			connStatus := e.(event.EvtPeerConnectednessChanged)
			if connStatus.Connectedness != network.NotConnected || !d.set.Contains(connStatus.Peer) {
				continue
			}
			d.connector.RestartBackoff(connStatus.Peer)
			d.set.Remove(connStatus.Peer)
			d.host.ConnManager().UntagPeer(connStatus.Peer, topic)
			t.Reset(defaultConnectionInterval)
		}
	}
}

// advertise is a utility function that persistently advertises a service through an Advertiser.
func (d *discovery) advertise(ctx context.Context) {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		ttl, err := d.disc.Advertise(ctx, topic)
		if err != nil {
			log.Debugf("Error advertising %s: %s", topic, err.Error())
			if ctx.Err() != nil {
				return
			}

			select {
			case <-timer.C:
				timer.Reset(interval)
				continue
			case <-ctx.Done():
				return
			}
		}

		select {
		case <-timer.C:
			timer.Reset(waitF(ttl))
		case <-ctx.Done():
			return
		}
	}
}
