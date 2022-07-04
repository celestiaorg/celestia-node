package share

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"

	core "github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

const (
	// PeersLimit is max amount of peers that will be discovered.
	PeersLimit = 5

	// peerWeight total weight of discovered peers.
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
		newLimitedSet(PeersLimit),
		h,
		d,
	}
}

// handlePeersFound receives peers and tries to establish a connection with them.
// Peer will be added to PeerCache if connection succeeds.
func (d *discovery) handlePeerFound(ctx context.Context, topic string, peer peer.AddrInfo) error {
	if peer.ID == d.host.ID() || len(peer.Addrs) == 0 || d.set.Contains(peer.ID) {
		return nil
	}
	err := d.set.TryAdd(peer.ID)
	if err != nil {
		return err
	}

	err = d.host.Connect(ctx, peer)
	if err != nil {
		log.Warn(err)
		d.set.Remove(peer.ID)
		return err
	}
	log.Debugw("added peer to set", "id", peer.ID)
	// add tag to protect peer of being killed by ConnManager
	d.host.ConnManager().TagPeer(peer.ID, topic, peerWeight)
	return nil
}

// findPeers starts peer discovery every 30 seconds until peer cache will not reach peersLimit.
// TODO(@vgonkivs): simplify when https://github.com/libp2p/go-libp2p/pull/1379 will be merged.
func (d *discovery) findPeers(ctx context.Context) {
	t := time.NewTicker(interval * 3)
	defer t.Stop()
	errg, errCtx := errgroup.WithContext(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			peers, err := d.disc.FindPeers(ctx, topic)
			if err != nil {
				log.Error(err)
				continue
			}
			for peer := range peers {
				peer := peer
				errg.Go(func() error {
					return d.handlePeerFound(errCtx, topic, peer)
				})
			}
			if err := errg.Wait(); err != nil {
				log.Warn(err) // informs that peers limit reached
				return
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
