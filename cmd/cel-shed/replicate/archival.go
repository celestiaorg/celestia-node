package replicate

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"

	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/discovery"
)

const (
	// archivalTopic is the discovery rendezvous tag used by archival nodes,
	// matching nodebuilder/share's archivalNodesTag.
	archivalTopic = "archival"

	// archivalProtocolVersion must match nodebuilder/share's protocolVersion
	// so that we rendezvous with celestia archival nodes on the same topic.
	archivalProtocolVersion = "v0.1.0"
)

// startArchivalDiscovery boots a Kademlia DHT in client mode, connects to the
// network's bootstrap peers, and starts a Discovery service on the "archival"
// topic. Discovered peers are reported into `pool` and logged.
//
// Returns a stop function that tears down the discovery service and DHT.
func startArchivalDiscovery(
	ctx context.Context,
	h host.Host,
	cfg ShrexFetchConfig,
	pool *peerPool,
) (func(), error) {
	bootstrappers, err := modp2p.BootstrappersFor(cfg.Network)
	if err != nil {
		return nil, fmt.Errorf("bootstrappers for %s: %w", cfg.Network, err)
	}
	log.Infow("connecting to bootstrappers", "count", len(bootstrappers))
	connectToBootstrappers(ctx, h, bootstrappers)

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	kdht, err := discovery.NewDHT(ctx, cfg.Network.String(), bootstrappers, h, ds, dht.ModeClient)
	if err != nil {
		return nil, fmt.Errorf("new dht: %w", err)
	}
	if err := kdht.Bootstrap(ctx); err != nil {
		_ = kdht.Close()
		return nil, fmt.Errorf("dht bootstrap: %w", err)
	}
	log.Infow("dht bootstrapped")

	rdisc := routingdisc.NewRoutingDiscovery(kdht)

	params := discovery.DefaultParameters()
	if cfg.DiscoveryLimit > 0 {
		params.PeersLimit = cfg.DiscoveryLimit
	}

	var counter struct {
		sync.Mutex
		n int
	}
	onUpdate := func(id peer.ID, isAdded bool) {
		if isAdded {
			pool.add(id)
			counter.Lock()
			counter.n++
			total := counter.n
			counter.Unlock()
			log.Infow("discovered archival peer", "peer", id.String(), "total", total)
		} else {
			pool.remove(id)
			log.Infow("archival peer removed", "peer", id.String())
		}
	}

	disc, err := discovery.NewDiscovery(
		params,
		h,
		rdisc,
		archivalTopic,
		archivalProtocolVersion,
		discovery.WithOnPeersUpdate(onUpdate),
	)
	if err != nil {
		_ = kdht.Close()
		return nil, fmt.Errorf("new discovery: %w", err)
	}
	if err := disc.Start(ctx); err != nil {
		_ = kdht.Close()
		return nil, fmt.Errorf("start discovery: %w", err)
	}

	stop := func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := disc.Stop(stopCtx); err != nil {
			log.Warnw("stop discovery", "err", err)
		}
		if err := kdht.Close(); err != nil {
			log.Warnw("close dht", "err", err)
		}
	}
	return stop, nil
}

// connectToBootstrappers dials the given bootstrappers concurrently, logging
// outcomes. Errors are non-fatal: as long as at least one connects, the DHT
// can bootstrap.
func connectToBootstrappers(ctx context.Context, h host.Host, boots []peer.AddrInfo) {
	var wg sync.WaitGroup
	for _, b := range boots {
		wg.Add(1)
		go func(b peer.AddrInfo) {
			defer wg.Done()
			dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err := h.Connect(dialCtx, b); err != nil {
				log.Warnw("bootstrap connect failed", "peer", b.ID.String(), "err", err)
				return
			}
			log.Infow("bootstrap connected", "peer", b.ID.String())
		}(b)
	}
	wg.Wait()
}
