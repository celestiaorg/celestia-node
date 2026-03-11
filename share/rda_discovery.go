package share

import (
	"context"
	"fmt"
	"sync"
	"time"

	libp2pDisc "github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	// rdaDiscoveryInterval is how often we re-advertise + re-discover peers.
	rdaDiscoveryInterval = 30 * time.Second

	// rdaAdvertiseTTL is how long our DHT advertisement is valid.
	rdaAdvertiseTTL = 5 * time.Minute

	// rdaDiscoveryLimit is the max peers we ask DHT for per namespace.
	rdaDiscoveryLimit = 50

	// rdaConnectTimeout is the timeout for each connection attempt.
	rdaConnectTimeout = 10 * time.Second
)

// RDADiscovery provides two mechanisms for a node to find its grid neighbors:
//
//  1. Bootstrap: on Start, connects directly to known bridge/bootstrap peers.
//     Once connected, the DHT can propagate to all other grid members.
//
//  2. DHT Rendezvous: advertises this node's grid position as namespaces
//     "rda/row/<N>" and "rda/col/<M>", and periodically queries those
//     namespaces to find and connect to same-row/col peers.
//
// After connections are established the PeerManager (via libp2p event bus)
// automatically classifies peers into row/col buckets.
type RDADiscovery struct {
	host           host.Host
	disc           libp2pDisc.Discovery
	gridManager    *RDAGridManager
	myPos          GridPosition
	bootstrapPeers []peer.AddrInfo
	interval       time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewRDADiscovery creates a new RDADiscovery.
//
//   - h:              this node's libp2p host
//   - disc:           a libp2p discovery.Discovery backed by the DHT
//     (e.g. routingdisc.NewRoutingDiscovery(dht))
//   - gm:             the shared RDAGridManager
//   - bootstrapPeers: addresses of known stable nodes (bridge nodes) to
//     connect to first, so DHT discovery can follow
func NewRDADiscovery(
	h host.Host,
	disc libp2pDisc.Discovery,
	gm *RDAGridManager,
	bootstrapPeers []peer.AddrInfo,
) *RDADiscovery {
	return &RDADiscovery{
		host:           h,
		disc:           disc,
		gridManager:    gm,
		myPos:          GetCoords(h.ID(), gm.GetGridDimensions()),
		bootstrapPeers: bootstrapPeers,
		interval:       rdaDiscoveryInterval,
	}
}

// Start connects to bootstrap peers and begins the advertising + discovery loop.
func (d *RDADiscovery) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	discCtx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel

	// Phase 1: connect to bootstrap peers immediately so we have at least
	// one peer to talk to before DHT discovery runs.
	if len(d.bootstrapPeers) > 0 {
		d.connectToBootstrap(ctx)
	}

	// Phase 2: run background loop for DHT-based rendezvous.
	d.wg.Add(1)
	go d.run(discCtx)

	log.Infof("RDA discovery started (row=%d, col=%d, bootstrap=%d)",
		d.myPos.Row, d.myPos.Col, len(d.bootstrapPeers))
	return nil
}

// Stop halts all discovery activity.
func (d *RDADiscovery) Stop(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
	d.wg.Wait()
	log.Infof("RDA discovery stopped")
	return nil
}

// run is the background goroutine that advertises + discovers peers.
func (d *RDADiscovery) run(ctx context.Context) {
	defer d.wg.Done()

	// Run immediately on start, then on each tick.
	d.advertise(ctx)
	d.discoverAndConnect(ctx)

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.advertise(ctx)
			d.discoverAndConnect(ctx)
		}
	}
}

// rowNS returns the DHT rendezvous namespace for a grid row.
func rowNS(row int) string { return fmt.Sprintf("rda/row/%d", row) }

// colNS returns the DHT rendezvous namespace for a grid column.
func colNS(col int) string { return fmt.Sprintf("rda/col/%d", col) }

// advertise registers this node under its grid namespaces in the DHT.
func (d *RDADiscovery) advertise(ctx context.Context) {
	ttl := libp2pDisc.TTL(rdaAdvertiseTTL)

	if _, err := d.disc.Advertise(ctx, rowNS(d.myPos.Row), ttl); err != nil {
		log.Debugf("RDA advertise row=%d failed: %v", d.myPos.Row, err)
	}
	if _, err := d.disc.Advertise(ctx, colNS(d.myPos.Col), ttl); err != nil {
		log.Debugf("RDA advertise col=%d failed: %v", d.myPos.Col, err)
	}
}

// discoverAndConnect queries DHT for same-row and same-col peers and connects.
func (d *RDADiscovery) discoverAndConnect(ctx context.Context) {
	d.findAndConnect(ctx, rowNS(d.myPos.Row))
	d.findAndConnect(ctx, colNS(d.myPos.Col))
}

// findAndConnect queries the DHT for ns and connects to all returned peers.
func (d *RDADiscovery) findAndConnect(ctx context.Context, ns string) {
	findCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	peerCh, err := d.disc.FindPeers(findCtx, ns, libp2pDisc.Limit(rdaDiscoveryLimit))
	if err != nil {
		log.Debugf("RDA findPeers [%s] failed: %v", ns, err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case pi, ok := <-peerCh:
			if !ok {
				return
			}
			if pi.ID == "" || pi.ID == d.host.ID() {
				continue
			}
			// Already connected — nothing to do.
			if d.host.Network().Connectedness(pi.ID) == network.Connected {
				continue
			}
			go d.connectPeer(pi, ns)
		}
	}
}

// connectPeer dials a peer discovered via DHT.
func (d *RDADiscovery) connectPeer(pi peer.AddrInfo, ns string) {
	ctx, cancel := context.WithTimeout(context.Background(), rdaConnectTimeout)
	defer cancel()

	if err := d.host.Connect(ctx, pi); err != nil {
		log.Debugf("RDA connect %s [%s] failed: %v", pi.ID, ns, err)
		return
	}
	log.Debugf("RDA connected to %s via [%s]", pi.ID, ns)
}

// connectToBootstrap connects directly to known bootstrap peers.
// Used during Start to get an initial foothold in the DHT network.
func (d *RDADiscovery) connectToBootstrap(ctx context.Context) {
	var wg sync.WaitGroup
	for _, bp := range d.bootstrapPeers {
		if bp.ID == d.host.ID() {
			continue
		}
		wg.Add(1)
		go func(p peer.AddrInfo) {
			defer wg.Done()
			dialCtx, cancel := context.WithTimeout(ctx, rdaConnectTimeout)
			defer cancel()
			if err := d.host.Connect(dialCtx, p); err != nil {
				log.Debugf("RDA bootstrap connect %s failed: %v", p.ID, err)
			} else {
				log.Infof("RDA bootstrap connected to %s", p.ID)
			}
		}(bp)
	}
	wg.Wait()
}

// GetMyRendezvousPoints returns the DHT namespaces this node advertises under.
func (d *RDADiscovery) GetMyRendezvousPoints() (rowNS, colNS string) {
	return fmt.Sprintf("rda/row/%d", d.myPos.Row),
		fmt.Sprintf("rda/col/%d", d.myPos.Col)
}

// SetInterval overrides the re-discovery interval (useful for tests).
func (d *RDADiscovery) SetInterval(interval time.Duration) {
	d.interval = interval
}
