package discovery

import (
	"context"
	"errors"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
)

const (
	peersLimit = 5
	peerWeight = 1000
)

var log = logging.Logger("discovery")

// Notifee allows to receive and store discovered peers.
type Notifee struct {
	cache *PeerCache
	host  host.Host
}

// NewNotifee constructs new Notifee.
func NewNotifee(cache *PeerCache, h host.Host) *Notifee {
	return &Notifee{
		cache,
		h,
	}
}

// HandlePeersFound receives peers and tries to establish a connection with them.
// Peer will be added to PeerCache if connection succeeds.
func (n *Notifee) HandlePeersFound(topic string, peers []peer.AddrInfo) error {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	for _, peer := range peers {
		if cancel != nil {
			cancel()
		}
		if n.cache.Size() == peersLimit {
			return errors.New("amount of peers reaches the limit")
		}

		if peer.ID == n.host.ID() || len(peer.Addrs) == 0 || n.cache.Has(peer.ID) {
			continue
		}
		if n.host.Network().Connectedness(peer.ID) != network.Connected {
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute*1)
			err := n.host.Connect(ctx, peer)
			if err != nil {
				log.Warn(err)
				continue
			}
		}
		log.Debugw("adding peer to cache", "id", peer.ID)
		n.host.ConnManager().TagPeer(peer.ID, topic, peerWeight)
		n.cache.Add(peer.ID)
		go n.emit(peer.ID, network.Connected)
	}

	if cancel != nil {
		cancel()
	}
	return nil
}

func (n *Notifee) emit(id peer.ID, state network.Connectedness) {
	emitter, err := n.host.EventBus().Emitter(&event.EvtPeerConnectednessChanged{})
	if err != nil {
		log.Warn(err)
		return
	}
	err = emitter.Emit(event.EvtPeerConnectednessChanged{Peer: id, Connectedness: state})
	if err != nil {
		log.Warn(err)
	}
}
