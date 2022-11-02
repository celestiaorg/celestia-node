package discovery

import (
	"context"
	"time"

	"github.com/celestiaorg/celestia-node/share/availability"
	logging "github.com/ipfs/go-log/v2"
	core "github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
)

var log = logging.Logger("share/discovery")

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

// Discovery combines advertise and discover services and allows to store discovered nodes.
type Discovery struct {
	params availability.DiscoveryParameters

	set       *limitedSet
	host      host.Host
	disc      core.Discovery
	connector *backoffConnector
}

// NewDiscovery constructs a new discovery.
func NewDiscovery(
	h host.Host,
	d core.Discovery,
	options ...availability.DiscOption,
) (*Discovery, error) {
	disc := &Discovery{
		params:    availability.DefaultDiscoveryParameters(),
		host:      h,
		disc:      d,
		connector: newBackoffConnector(h, defaultBackoffFactory),
	}

	for _, applyOpt := range options {
		applyOpt(disc)
	}

	err := disc.params.Validate()
	if err != nil {
		return nil, err
	}

	disc.set = newLimitedSet(disc.params.PeersLimit)

	return disc, nil
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
	log.Debugw("added peer to set", "id", peer.ID)
	// add tag to protect peer of being killed by ConnManager
	d.host.ConnManager().TagPeer(peer.ID, topic, peerWeight)
}

// EnsurePeers ensures we always have 'peerLimit' connected peers.
// It starts peer discovery every 30 seconds until peer cache reaches peersLimit.
// Discovery is restarted if any previously connected peers disconnect.
func (d *Discovery) EnsurePeers(ctx context.Context) {
	if d.params.PeersLimit == 0 {
		log.Warn("peers limit is set to 0. Skipping discovery...")
		return
	}
	// subscribe on Event Bus in order to catch disconnected peers and restart the discovery
	sub, err := d.host.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{})
	if err != nil {
		log.Error(err)
		return
	}
	go d.connector.GC(ctx)

	t := time.NewTicker(d.params.DiscoveryInterval)
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
			if uint(d.set.Size()) == d.params.PeersLimit {
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
		case e := <-sub.Out():
			// listen to disconnect event to remove peer from set and reset backoff time
			// reset timer in order to restart the discovery, once stored peer is disconnected
			connStatus := e.(event.EvtPeerConnectednessChanged)
			if connStatus.Connectedness == network.NotConnected {
				if d.set.Contains(connStatus.Peer) {
					d.connector.RestartBackoff(connStatus.Peer)
					d.set.Remove(connStatus.Peer)
					d.host.ConnManager().UntagPeer(connStatus.Peer, topic)
					t.Reset(d.params.DiscoveryInterval)
				}
			}
		}
	}
}

// Advertise is a utility function that persistently advertises a service through an Advertiser.
func (d *Discovery) Advertise(ctx context.Context) {
	timer := time.NewTimer(d.params.AdvertiseInterval)
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
				timer.Reset(d.params.AdvertiseInterval)
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

// SetParam sets configurable parameters for the Discovery implementation of `Discoverable`
// per the Parameterizable interface
func (d *Discovery) SetParam(key string, value any) {
	switch key {
	case "PeersList":
		ivalue, _ := value.(uint)
		d.params.PeersLimit = ivalue
	case "DiscoveryInterval":
		ivalue, _ := value.(time.Duration)
		d.params.DiscoveryInterval = ivalue
	case "AdvertiseInterval":
		ivalue, _ := value.(time.Duration)
		d.params.AdvertiseInterval = ivalue
	default:
		log.Warn("LightAvailability tried to SetParam for unknown configuration key: %s", key)
	}
}
