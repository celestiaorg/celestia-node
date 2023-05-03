package discovery

import (
	"context"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/host/eventbus"
	"golang.org/x/sync/errgroup"
)

var log = logging.Logger("share/discovery")

const (
	topic = "full"

	// eventbusBufSize is the size of the buffered channel to handle
	// events in libp2p. We specify a larger buffer size for the channel
	// to avoid overflowing and blocking subscription during disconnection bursts.
	// (by default it is 16)
	eventbusBufSize = 64

	// findPeersStuckWarnDelay is the duration after which discover will log an error message to
	// notify that it is stuck.
	findPeersStuckWarnDelay = time.Minute

	// defaultRetryTimeout defines time interval between discovery attempts.
	defaultRetryTimeout = time.Second
)

// waitF calculates time to restart announcing.
var waitF = func(ttl time.Duration) time.Duration {
	return 7 * ttl / 8
}

type Parameters struct {
	// PeersLimit defines the soft limit of FNs to connect to via discovery.
	// Set 0 to disable.
	PeersLimit uint
	// AdvertiseInterval is a interval between advertising sessions.
	// Set -1 to disable.
	// NOTE: only full and bridge can advertise themselves.
	AdvertiseInterval time.Duration
	// discoveryRetryTimeout is an interval between discovery attempts
	// when we discovered lower than PeersLimit peers.
	// Set -1 to disable.
	discoveryRetryTimeout time.Duration
}

func (p Parameters) withDefaults() Parameters {
	def := DefaultParameters()
	if p.AdvertiseInterval == 0 {
		p.AdvertiseInterval = def.AdvertiseInterval
	}
	if p.discoveryRetryTimeout == 0 {
		p.discoveryRetryTimeout = defaultRetryTimeout
	}
	return p
}

func DefaultParameters() Parameters {
	return Parameters{
		PeersLimit:        5,
		AdvertiseInterval: time.Hour * 8,
	}
}

// Discovery combines advertise and discover services and allows to store discovered nodes.
// TODO: The code here gets horribly hairy, so we should refactor this at some point
type Discovery struct {
	params Parameters

	set            *limitedSet
	host           host.Host
	disc           discovery.Discovery
	connector      *backoffConnector
	onUpdatedPeers OnUpdatedPeers

	triggerDisc chan struct{}

	cancel context.CancelFunc
}

type OnUpdatedPeers func(peerID peer.ID, isAdded bool)

// NewDiscovery constructs a new discovery.
func NewDiscovery(
	h host.Host,
	d discovery.Discovery,
	params Parameters,
) *Discovery {
	return &Discovery{
		params:         params.withDefaults(),
		set:            newLimitedSet(params.PeersLimit),
		host:           h,
		disc:           d,
		connector:      newBackoffConnector(h, defaultBackoffFactory),
		onUpdatedPeers: func(peer.ID, bool) {},
		triggerDisc:    make(chan struct{}),
	}
}

func (d *Discovery) Start(context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel

	if d.params.PeersLimit == 0 {
		log.Warn("peers limit is set to 0. Skipping discovery...")
		return nil
	}

	sub, err := d.host.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{}, eventbus.BufSize(eventbusBufSize))
	if err != nil {
		return fmt.Errorf("subscribing for connection events: %w", err)
	}

	go d.discoveryLoop(ctx)
	go d.disconnectsLoop(ctx, sub)
	go d.connector.GC(ctx)
	return nil
}

func (d *Discovery) Stop(context.Context) error {
	d.cancel()
	return nil
}

// WithOnPeersUpdate chains OnPeersUpdate callbacks on every update of discovered peers list.
func (d *Discovery) WithOnPeersUpdate(f OnUpdatedPeers) {
	prev := d.onUpdatedPeers
	d.onUpdatedPeers = func(peerID peer.ID, isAdded bool) {
		prev(peerID, isAdded)
		f(peerID, isAdded)
	}
}

// Peers provides a list of discovered peers in the "full" topic.
// If Discovery hasn't found any peers, it blocks until at least one peer is found.
func (d *Discovery) Peers(ctx context.Context) ([]peer.ID, error) {
	return d.set.Peers(ctx)
}

// Advertise is a utility function that persistently advertises a service through an Advertiser.
// TODO: Start advertising only after the reachability is confirmed by AutoNAT
func (d *Discovery) Advertise(ctx context.Context) {
	if d.params.AdvertiseInterval == -1 {
		return
	}

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

		log.Debugf("advertised")
		select {
		case <-timer.C:
			timer.Reset(waitF(ttl))
		case <-ctx.Done():
			return
		}
	}
}

// discoveryLoop ensures we always have '~peerLimit' connected peers.
// It starts peer discovery per request and restarts the process until the soft limit reached.
func (d *Discovery) discoveryLoop(ctx context.Context) {
	t := time.NewTicker(d.params.discoveryRetryTimeout)
	defer t.Stop()
	for {
		// drain all previous ticks from channel
		drainChannel(t.C)
		select {
		case <-t.C:
			found := d.discover(ctx)
			if !found {
				// rerun discovery if amount of peers didn't reach the limit
				continue
			}
		case <-ctx.Done():
			return
		}

		select {
		case <-d.triggerDisc:
		case <-ctx.Done():
			return
		}
	}
}

// disconnectsLoop listen for disconnect events and ensures Discovery state
// is updated.
func (d *Discovery) disconnectsLoop(ctx context.Context, sub event.Subscription) {
	defer sub.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-sub.Out():
			if !ok {
				log.Error("connection subscription was closed unexpectedly")
				return
			}

			if evnt := e.(event.EvtPeerConnectednessChanged); evnt.Connectedness == network.NotConnected {
				if !d.set.Contains(evnt.Peer) {
					continue
				}

				d.host.ConnManager().Unprotect(evnt.Peer, topic)
				d.connector.Backoff(evnt.Peer)
				d.set.Remove(evnt.Peer)
				d.onUpdatedPeers(evnt.Peer, false)
				log.Debugw("removed peer from the peer set",
					"peer", evnt.Peer, "status", evnt.Connectedness.String())

				if d.set.Size() < d.set.Limit() {
					// trigger discovery
					select {
					case d.triggerDisc <- struct{}{}:
					default:
					}
				}
			}
		}
	}
}

// discover finds new peers and reports whether it succeeded.
func (d *Discovery) discover(ctx context.Context) bool {
	size := d.set.Size()
	want := d.set.Limit() - size
	if want == 0 {
		log.Debugw("reached soft peer limit, skipping discovery", "size", size)
		return true
	}
	log.Infow("discovering peers", "want", want)

	// we use errgroup as it provide limits
	var wg errgroup.Group
	// limit to minimize chances of overreaching the limit
	wg.SetLimit(int(d.set.Limit()))
	defer wg.Wait() //nolint:errcheck

	// stop discovery when we are done
	findCtx, findCancel := context.WithCancel(ctx)
	defer findCancel()

	peers, err := d.disc.FindPeers(findCtx, topic)
	if err != nil {
		log.Error("unable to start discovery", "err", err)
		return false
	}

	ticker := time.NewTicker(findPeersStuckWarnDelay)
	defer ticker.Stop()
	for {
		ticker.Reset(findPeersStuckWarnDelay)
		// drain all previous ticks from channel
		drainChannel(ticker.C)
		select {
		case <-findCtx.Done():
			return true
		case <-ticker.C:
			log.Warn("wasn't able to find new peers for long time")
			continue
		case p, ok := <-peers:
			if !ok {
				log.Debugw("discovery channel closed", "find_is_canceled", findCtx.Err() != nil)
				return d.set.Size() >= d.set.Limit()
			}

			peer := p
			wg.Go(func() error {
				if findCtx.Err() != nil {
					log.Debug("find has been canceled, skip peer")
					return nil
				}

				// we don't pass findCtx so that we don't cancel in progress connections
				// that are likely to be valuable
				if !d.handleDiscoveredPeer(ctx, peer) {
					return nil
				}

				size := d.set.Size()
				log.Debugw("found peer", "peer", peer.ID, "found_amount", size)
				if size < d.set.Limit() {
					return nil
				}

				log.Infow("discovered wanted peers", "amount", size)
				findCancel()
				return nil
			})
		}
	}
}

// handleDiscoveredPeer adds peer to the internal if can connect or is connected.
// Report whether it succeeded.
func (d *Discovery) handleDiscoveredPeer(ctx context.Context, peer peer.AddrInfo) bool {
	logger := log.With("peer", peer.ID)
	switch {
	case peer.ID == d.host.ID():
		logger.Debug("skip handle: self discovery")
		return false
	case len(peer.Addrs) == 0:
		logger.Debug("skip handle: empty address list")
		return false
	case d.set.Size() >= d.set.Limit():
		logger.Debug("skip handle: enough peers found")
		return false
	case d.connector.HasBackoff(peer.ID):
		logger.Debug("skip handle: backoff")
		return false
	}

	switch d.host.Network().Connectedness(peer.ID) {
	case network.Connected:
		d.connector.Backoff(peer.ID) // we still have to backoff the connected peer
	case network.NotConnected:
		err := d.connector.Connect(ctx, peer)
		if err != nil {
			logger.Debugw("unable to connect", "err", err)
			return false
		}
	default:
		panic("unknown connectedness")
	}

	if !d.set.Add(peer.ID) {
		logger.Debug("peer is already in discovery set")
		return false
	}
	d.onUpdatedPeers(peer.ID, true)
	logger.Debug("added peer to set")

	// tag to protect peer from being killed by ConnManager
	// NOTE: This is does not protect from remote killing the connection.
	//  In the future, we should design a protocol that keeps bidirectional agreement on whether
	//  connection should be kept or not, similar to mesh link in GossipSub.
	d.host.ConnManager().Protect(peer.ID, topic)
	return true
}

func drainChannel(c <-chan time.Time) {
	for {
		select {
		case <-c:
		default:
			return
		}
	}
}
