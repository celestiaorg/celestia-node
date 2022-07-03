package share

import (
	"context"
	"errors"
	"time"

	"github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
)

const (
	// PeersLimit is max amount of peers that will be discovered
	PeersLimit = 5

	// peerWeight total weight of discovered peers
	peerWeight = 1000
	// connectionTimeout is timeout given to connect to discovered peer
	connectionTimeout = time.Minute
)

// discoverer finds and stores discovered full nodes.
type discoverer struct {
	set  *LimitedSet
	host host.Host
	disc discovery.Discoverer
}

// newDiscoverer constructs new Discoverer.
func newDiscoverer(set *LimitedSet, h host.Host, d discovery.Discoverer) *discoverer {
	return &discoverer{
		set,
		h,
		d,
	}
}

// handlePeersFound receives peers and tries to establish a connection with them.
// Peer will be added to PeerCache if connection succeeds.
func (d *discoverer) handlePeersFound(topic string, peers []peer.AddrInfo) error {
	for _, peer := range peers {
		if d.set.Size() == PeersLimit {
			return errors.New("amount of peers reaches the limit")
		}

		if peer.ID == d.host.ID() || len(peer.Addrs) == 0 || d.set.Contains(peer.ID) {
			continue
		}
		err := d.set.TryAdd(peer.ID)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
		err = d.host.Connect(ctx, peer)
		cancel()
		if err != nil {
			log.Warn(err)
			d.set.Remove(peer.ID)
			continue
		}
		log.Debugw("added peer to set", "id", peer.ID)
		d.host.ConnManager().TagPeer(peer.ID, topic, peerWeight)
	}
	return nil
}

// FindPeers starts peer discovery every 30 seconds until peer cache will not reach peersLimit.
// TODO(@vgonkivs): simplify when https://github.com/libp2p/go-libp2p/pull/1379 will be merged.
func (d *discoverer) findPeers(ctx context.Context) {
	t := time.NewTicker(interval * 3)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			peers, err := util.FindPeers(ctx, d.disc, topic)
			if err != nil {
				log.Debug(err)
				continue
			}
			if err = d.handlePeersFound(topic, peers); err != nil {
				log.Info(err) // informs that peers limit reached
				return
			}
		}
	}
}
