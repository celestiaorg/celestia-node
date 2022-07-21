package share

import (
	"context"
	"time"

	core "github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

const (
	// peerWeight is a weight of discovered peers.
	// peerWeight is a number that will be assigned to all discovered full nodes,
	// so ConnManager will not break a connection with them.
	peerWeight = 1000
	topic      = "full"
)

// waitF calculates time to restart announcing.
var waitF = func(ttl time.Duration) time.Duration {
	return 7 * ttl / 8
}

// discovery combines advertise and discover services and allows to store discovered nodes.
type discovery struct {
	set  *limitedSet
	host host.Host
	disc core.Discovery
	// peersLimit is max amount of peers that will be discovered.
	peersLimit uint
	// discInterval is an interval between discovery sessions.
	discoveryInterval time.Duration
	// advertiseInterval is an interval between advertising sessions.
	advertiseInterval time.Duration
}

// NewDiscovery constructs a new discovery.
func NewDiscovery(
	h host.Host,
	d core.Discovery,
	peersLimit uint,
	discInterval,
	advertiseInterval time.Duration,
) *discovery { //nolint:revive
	return &discovery{
		newLimitedSet(peersLimit),
		h,
		d,
		peersLimit,
		discInterval,
		advertiseInterval,
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

// findPeers starts peer discovery every discoveryInterval until peer cache will not reach peersLimit.
func (d *discovery) findPeers(ctx context.Context) {
	if d.peersLimit == 0 {
		log.Warn("peers limit is set to 0. Skipping discovery...")
		return
	}
	t := time.NewTicker(d.discoveryInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			// return if limit was reached to stop the loop and finish discovery
			if d.set.Size() >= d.peersLimit {
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
	timer := time.NewTimer(d.advertiseInterval)
	for {
		ttl, err := d.disc.Advertise(ctx, topic)
		if err != nil {
			log.Debugf("Error advertising %s: %s", topic, err.Error())
			if ctx.Err() != nil {
				return
			}

			select {
			case <-timer.C:
				timer.Reset(d.advertiseInterval)
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
