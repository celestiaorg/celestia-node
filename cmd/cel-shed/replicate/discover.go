package replicate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/cmd/cel-shed/replicate/headers"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

// DiscoverConfig configures a discover-archival run.
type DiscoverConfig struct {
	Network        modp2p.Network
	Timeout        time.Duration // total time to keep discovering before reporting
	DiscoveryLimit uint          // soft cap for the discovery peer pool
	LogLevel       string
	JSON           bool // emit results as JSON instead of a text table
}

func (c DiscoverConfig) Validate() error {
	if c.Timeout < time.Second {
		return fmt.Errorf("timeout must be >= 1s, got %s", c.Timeout)
	}
	return nil
}

// discoveredPeer is one archival peer found during discovery.
type discoveredPeer struct {
	ID    string   `json:"id"`
	Addrs []string `json:"addrs"`
}

// RunDiscoverArchival boots an ephemeral libp2p host, joins the network's DHT,
// and listens on the "archival" discovery topic for the configured timeout,
// accumulating every archival peer it sees. It then prints the peers (with the
// multiaddrs learned for them) as a text table or JSON.
//
// This is read-only: it advertises nothing and fetches no data. Peers churn in
// and out of the live pool, so results are the cumulative union of everything
// seen during the window, not just the peers connected at the end.
func RunDiscoverArchival(ctx context.Context, cfg DiscoverConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.LogLevel != "" {
		_ = logging.SetLogLevel("cmd-shed/replicate", cfg.LogLevel)
	}

	h, err := headers.NewReplicatorHost()
	if err != nil {
		return fmt.Errorf("new libp2p host: %w", err)
	}
	defer h.Close()
	log.Infow("libp2p host ready", "peer", h.ID().String())

	pool := newPeerPool()
	log.Infow("starting archival discovery",
		"network", cfg.Network, "topic", archivalTopic, "timeout", cfg.Timeout)
	stopDisc, err := startArchivalDiscovery(ctx, h, ShrexFetchConfig{
		Network:        cfg.Network,
		DiscoveryLimit: cfg.DiscoveryLimit,
	}, pool)
	if err != nil {
		return fmt.Errorf("start discovery: %w", err)
	}
	defer stopDisc()

	// Accumulate the cumulative set of peers seen, since discovery drops peers
	// from the live pool as they churn. Poll the pool until the window elapses
	// (or the process is interrupted).
	seen := make(map[peer.ID]struct{})
	deadline := time.NewTimer(cfg.Timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	interrupted := false
loop:
	for {
		for _, id := range pool.snapshot() {
			seen[id] = struct{}{}
		}
		select {
		case <-deadline.C:
			break loop
		case <-ctx.Done():
			interrupted = true
			break loop
		case <-ticker.C:
		}
	}
	// Final sweep in case peers arrived between the last tick and the deadline.
	for _, id := range pool.snapshot() {
		seen[id] = struct{}{}
	}

	if interrupted && !errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}

	peers := make([]discoveredPeer, 0, len(seen))
	for id := range seen {
		info := h.Peerstore().PeerInfo(id)
		p2pAddrs, err := peer.AddrInfoToP2pAddrs(&info)
		addrs := make([]string, 0, len(p2pAddrs))
		if err == nil {
			for _, a := range p2pAddrs {
				addrs = append(addrs, a.String())
			}
		}
		peers = append(peers, discoveredPeer{ID: id.String(), Addrs: addrs})
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].ID < peers[j].ID })

	log.Infow("discovery finished", "peers", len(peers), "interrupted", interrupted)
	return printDiscovered(cfg, peers)
}

func printDiscovered(cfg DiscoverConfig, peers []discoveredPeer) error {
	if cfg.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(peers)
	}

	fmt.Printf("discovered %d archival peer(s) on %s:\n", len(peers), cfg.Network)
	for _, p := range peers {
		if len(p.Addrs) == 0 {
			// No dialable multiaddr learned; the peer id is still useful.
			fmt.Printf("%s\t(no known addresses)\n", p.ID)
			continue
		}
		for _, a := range p.Addrs {
			fmt.Printf("%s\n", a)
		}
	}
	return nil
}
