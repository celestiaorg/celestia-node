package discovery

import (
	"context"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/host/eventbus"
)

var log = logging.Logger("share/discovery")

const (
	// peerWeight is a weight of discovered peers.
	// peerWeight is a number that will be assigned to all discovered full nodes,
	// so ConnManager will not break a connection with them.
	peerWeight = 1000
	topic      = "full"

	// eventbusBufSize is the size of the buffered channel to handle
	// events in libp2p
	eventbusBufSize = 32
)

// waitF calculates time to restart announcing.
var waitF = func(ttl time.Duration) time.Duration {
	return 7 * ttl / 8
}

// Discovery combines advertise and discover services and allows to store discovered nodes.
type Discovery struct {
	set       *limitedSet
	host      host.Host
	disc      discovery.Discovery
	connector *backoffConnector
	// peersLimit is max amount of peers that will be discovered during a discovery session.
	peersLimit uint
	// discInterval is an interval between discovery sessions.
	discoveryInterval time.Duration
	// advertiseInterval is an interval between advertising sessions.
	advertiseInterval time.Duration
	// onUpdatedPeers will be called on peer set changes
	onUpdatedPeers OnUpdatedPeers
	// ensureIsRunning allows only one ensurePeers process to be running
	ensurePeersOnce sync.Once
}

type OnUpdatedPeers func(peerID peer.ID, isAdded bool)

// NewDiscovery constructs a new discovery.
func NewDiscovery(
	h host.Host,
	d discovery.Discovery,
	peersLimit uint,
	discInterval,
	advertiseInterval time.Duration,
) *Discovery {
	return &Discovery{
		newLimitedSet(peersLimit),
		h,
		d,
		newBackoffConnector(h, defaultBackoffFactory),
		peersLimit,
		discInterval,
		advertiseInterval,
		func(peer.ID, bool) {},
		sync.Once{},
	}
}

// WithOnPeersUpdate adds OnPeersUpdate callback call on every update of discovered peers list.
func (d *Discovery) WithOnPeersUpdate(f OnUpdatedPeers) {
	d.onUpdatedPeers = func(peerID peer.ID, isAdded bool) {
		d.onUpdatedPeers(peerID, isAdded)
		f(peerID, isAdded)
	}
}

// handlePeersFound receives peers and tries to establish a connection with them.
// Peer will be added to PeerCache if connection succeeds.
func (d *Discovery) handlePeerFound(ctx context.Context, topic string, peer peer.AddrInfo) {
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
		log.Debug(err)
		d.set.Remove(peer.ID)
		return
	}

	d.onUpdatedPeers(peer.ID, true)
	log.Debugw("added peer to set", "id", peer.ID)
	// add tag to protect peer of being killed by ConnManager
	d.host.ConnManager().TagPeer(peer.ID, topic, peerWeight)
}

// EnsurePeers ensures we always have 'peerLimit' connected peers.
// It starts peer discovery every 30 seconds until peer cache reaches peersLimit.
// Discovery is restarted if any previously connected peers disconnect.
func (d *Discovery) EnsurePeers(ctx context.Context) {
	d.ensurePeersOnce.Do(func() {
		go d.ensurePeers(ctx)
	})
}

func (d *Discovery) ensurePeers(ctx context.Context) {
	if d.peersLimit == 0 {
		log.Warn("peers limit is set to 0. Skipping discovery...")
		return
	}
	// subscribe on EventBus in order to catch disconnected peers and restart
	// the discovery. We specify a larger buffer size for the channel where
	// EvtPeerConnectednessChanged events are sent (by default it is 16, we
	// specify 32) to avoid any blocks on writing to the full channel.
	sub, err := d.host.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{}, eventbus.BufSize(eventbusBufSize))
	if err != nil {
		log.Error(err)
		return
	}
	go d.connector.GC(ctx)

	t := time.NewTicker(d.discoveryInterval)
	defer func() {
		t.Stop()
		if err = sub.Close(); err != nil {
			log.Error(err)
		}
	}()

	// starting to listen to subscriptions async will help us to avoid any blocking
	// in the case when we will not have the needed amount of FNs and will be blocked in `FindPeers`.
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Debug("Context canceled. Finish listening for connectedness events.")
				return
			case e, ok := <-sub.Out():
				if !ok {
					log.Debug("Subscription for connectedness events is closed.")
					return
				}
				// listen to disconnect event to remove peer from set and reset backoff time
				// reset timer in order to restart the discovery, once stored peer is disconnected
				connStatus := e.(event.EvtPeerConnectednessChanged)
				if connStatus.Connectedness == network.NotConnected {
					if d.set.Contains(connStatus.Peer) {
						log.Debugw("removing the peer from the peer set",
							"peer", connStatus.Peer, "status", connStatus.Connectedness.String())
						d.connector.RestartBackoff(connStatus.Peer)
						d.set.Remove(connStatus.Peer)
						d.onUpdatedPeers(connStatus.Peer, false)
						d.host.ConnManager().UntagPeer(connStatus.Peer, topic)
						t.Reset(d.discoveryInterval)
					}
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Info("Context canceled. Finishing peer discovery")
			return
		case <-t.C:
			if uint(d.set.Size()) == d.peersLimit {
				// stop ticker if we have reached the limit
				t.Stop()
				continue
			}
			peers, err := d.disc.FindPeers(ctx, topic)
			if err != nil {
				log.Error(err)
				continue
			}
			for p := range peers {
				go d.handlePeerFound(ctx, topic, p)
			}
		}
	}
}

// Advertise is a utility function that persistently advertises a service through an Advertiser.
func (d *Discovery) Advertise(ctx context.Context) {
	timer := time.NewTimer(d.advertiseInterval)
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

// Peers provides a list of discovered peers in the "full" topic.
// If Discovery hasn't found any peers, it blocks until at least one peer is found.
func (d *Discovery) Peers(ctx context.Context) ([]peer.ID, error) {
	return d.set.Peers(ctx)
}
