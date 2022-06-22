package discovery

import (
	"context"
	"errors"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

const (
	// PeersLimit is max amount of peers that will be discovered
	PeersLimit = 5

	// peerWeight total weight of discovered peers
	peerWeight = 1000
	// connectionTimeout is timeout given to connect to discovered peer
	connectionTimeout = time.Minute
)

var log = logging.Logger("discovery")

// Discoverer finds and stores discovered full nodes.
type Discoverer struct {
	set  *LimitedSet
	host host.Host
	disc discovery.Discoverer
}

// NewDiscoverer constructs new Discoverer.
func NewDiscoverer(set *LimitedSet, h host.Host, d discovery.Discoverer) *Discoverer {
	return &Discoverer{
		set,
		h,
		d,
	}
}

// handlePeersFound receives peers and tries to establish a connection with them.
// Peer will be added to PeerCache if connection succeeds.
func (d *Discoverer) handlePeersFound(topic string, peers []peer.AddrInfo) error {
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
