package share

import (
	"context"
	"time"

	core "github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

const (
	// peersLimit is max amount of peers that will be discovered.
	peersLimit = 5

	// peerWeight is a weight of discovered peers.
	// peerWeight is a number that will be assigned to all discovered full nodes,
	// so ConnManager will not break a connection with them.
	peerWeight = 1000
	topic      = "full"
	interval   = time.Second * 10
)

// waitF calculates time to restart announcing.
var waitF = func(ttl time.Duration) time.Duration {
	return 7 * ttl / 8
}

// discoverer used to protect light nodes of being advertised as they support only peer discovery.
type discoverer interface {
	findPeers(ctx context.Context)
}

// discovery combines advertise and discover services and allows to store discovered nodes.
type discovery struct {
	set  *limitedSet
	host host.Host
	disc core.Discovery
}

// newDiscovery constructs a new discovery.
func newDiscovery(h host.Host, d core.Discovery) *discovery {
	return &discovery{
		newLimitedSet(peersLimit),
		h,
		d,
	}
}

// handlePeersFound receives peers and tries to establish a connection with them.
// Peer will be added to PeerCache if connection succeeds.
func (d *discovery) handlePeerFound(ctx context.Context, topic string, peer peer.AddrInfo) {
	if peer.ID == d.host.ID() || len(peer.Addrs) == 0 {
		return
	}
	err := d.set.TryAdd(peer.ID)
	if err != nil {
		log.Debug(err)
		return
	}

	err = d.host.Connect(ctx, peer)
	if err != nil {
		log.Warn(err)
		d.set.Remove(peer.ID)
		return
	}
	log.Debugw("added peer to set", "id", peer.ID)
	// add tag to protect peer of being killed by ConnManager
	d.host.ConnManager().TagPeer(peer.ID, topic, peerWeight)
}

// findPeers starts peer discovery every 30 seconds until peer cache will not reach peersLimit.
// TODO (@vgonkivs): simplify when https://github.com/libp2p/go-libp2p/pull/1379 will be merged.
func (d *discovery) findPeers(ctx context.Context) {
	t := time.NewTicker(interval * 3)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			// return if limit was reached to stop the loop and finish discovery
			if d.set.Size() >= peersLimit {
				return
			}
			peers, err := d.disc.FindPeers(ctx, topic)
			if err != nil {
				log.Error(err)
				continue
			}
			for peer := range peers {
				go d.handlePeerFound(ctx, topic, peer)
			}
		}
	}
}

// advertise is a utility function that persistently advertises a service through an Advertiser.
func (d *discovery) advertise(ctx context.Context) {
	timer := time.NewTimer(interval)
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
