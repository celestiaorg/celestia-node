package discovery

import (
	"context"
	"errors"
	"time"

	logging "github.com/ipfs/go-log/v2"

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

// Notifee allows to receive and store discovered peers.
type Notifee struct {
	set  *LimitedSet
	host host.Host
}

// NewNotifee constructs new Notifee.
func NewNotifee(set *LimitedSet, h host.Host) *Notifee {
	return &Notifee{
		set,
		h,
	}
}

// HandlePeersFound receives peers and tries to establish a connection with them.
// Peer will be added to PeerCache if connection succeeds.
func (n *Notifee) HandlePeersFound(topic string, peers []peer.AddrInfo) error {
	for _, peer := range peers {
		if n.set.Size() == PeersLimit {
			return errors.New("amount of peers reaches the limit")
		}

		if peer.ID == n.host.ID() || len(peer.Addrs) == 0 || n.set.Contains(peer.ID) {
			continue
		}
		err := n.set.TryAdd(peer.ID)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
		err = n.host.Connect(ctx, peer)
		cancel()
		if err != nil {
			log.Warn(err)
			n.set.Remove(peer.ID)
			continue
		}
		log.Debugw("added peer to set", "id", peer.ID)
		n.host.ConnManager().TagPeer(peer.ID, topic, peerWeight)
	}
	return nil
}
