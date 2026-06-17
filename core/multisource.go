package core

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc"
)

// taggedSource pairs a source endpoint with its address (conn.Target). It is the
// constructor input unit; MultiSource itself stores sources keyed by addr.
type taggedSource struct {
	fetcher blockSource
	addr    string
}

// MultiSource fans several core endpoints into one new-block stream. Each
// endpoint stays subscribed independently (the underlying BlockFetcher already
// resubscribes forever on its own), so a single failing endpoint cannot stall
// the others. Duplicate heights across sources are expected and must be deduplicated by
// the consumer.
type MultiSource struct {
	// sources keyed by addr (conn.Target). The announcing source's addr travels
	// on every BlockEvent, so GetSignedBlockFrom/IsSyncingFrom resolve back to it
	// by addr — no indices that pruning in Verify would shift. Addr is a reliable
	// id: one addr is one core endpoint, and duplicate endpoints are rejected at
	// config validation, so the map never silently collapses distinct sources.
	sources map[string]blockSource
}

// NewMultiSource builds a MultiSource over the given gRPC connections. With a
// single connection it behaves equivalently to a single-source BlockFetcher.
func NewMultiSource(grpcClients ...*grpc.ClientConn) *MultiSource {
	sources := make([]taggedSource, len(grpcClients))
	for i, client := range grpcClients {
		sources[i] = taggedSource{fetcher: NewBlockFetcher(client), addr: client.Target()}
	}
	return newMultiSource(sources...)
}

// newMultiSource is the internal constructor used by NewMultiSource and tests.
func newMultiSource(sources ...taggedSource) *MultiSource {
	byAddr := make(map[string]blockSource, len(sources))
	for _, s := range sources {
		byAddr[s.addr] = s.fetcher
	}
	return &MultiSource{sources: byAddr}
}

// Verify checks every source against the expected network and keeps only those
// that confirmed the expected chain ID. Both wrong-chain AND unreachable
// sources are pruned (logged with their address): sources are operator-curated
// endpoints, so an endpoint that cannot vouch for its network at startup has no
// business in the active set — letting it join later would defer a wrong-chain
// failure to a mid-run panic instead of a startup log. The operator restores a
// pruned endpoint by fixing it and restarting. It errors if no source could be
// confirmed, so a fully misconfigured or unreachable set refuses to start. The
// expected chain ID must be set: a node must know which network it serves.
//
// Verify mutates m.sources and is safe only because Listener.Start calls it
// synchronously before SubscribeNewBlockEvent spawns goroutines and before
// ChainID/IsSyncingFrom run. Pruning is keyed by addr, so it no longer matters
// that indices would shift — but concurrent mutation would still race the reads.
func (m *MultiSource) Verify(ctx context.Context, expected string) error {
	if expected == "" {
		return fmt.Errorf("multisource: expected chain ID must be configured")
	}

	// Snapshot the map into a slice so each goroutine writes its own indexed
	// result slot (a map can't be range-indexed for that). Query every source's
	// chain ID concurrently so a slow or unreachable endpoint doesn't serialize
	// startup behind it: total latency is the slowest source, not the sum. The wg
	// barrier alone orders the writes before the reads below — no mutex.
	type entry struct {
		addr string
		src  blockSource
	}
	entries := make([]entry, 0, len(m.sources))
	for addr, src := range m.sources {
		entries = append(entries, entry{addr: addr, src: src})
	}

	ids := make([]string, len(entries))
	errs := make([]error, len(entries))
	var wg sync.WaitGroup
	for i, e := range entries {
		wg.Add(1)
		go func(i int, e entry) {
			defer wg.Done()
			ids[i], errs[i] = e.src.ChainID(ctx)
		}(i, e)
	}
	wg.Wait()

	// Assemble the surviving set sequentially: free of shared mutation now that
	// all goroutines have returned.
	kept := make(map[string]blockSource, len(entries))
	for i, e := range entries {
		switch {
		case errs[i] != nil:
			log.Errorw("multisource: dropping unverifiable source",
				"source", e.addr, "err", errs[i])
		case ids[i] != expected:
			log.Errorw("multisource: dropping endpoint on wrong network",
				"source", e.addr, "expected", expected, "received", ids[i])
		default:
			kept[e.addr] = e.src
		}
	}
	m.sources = kept
	if len(kept) == 0 {
		return fmt.Errorf("multisource: no source confirmed on expected network %q", expected)
	}
	return nil
}

// SubscribeNewBlockEvent fans every source's subscription into one channel,
// closed once all source goroutines exit (i.e. ctx is canceled). It forwards
// BlockEvents, not full blocks: each event is tagged with its source's addr so
// the consumer fetches the block once from the announcing peer via
// GetSignedBlockFrom instead of every source downloading it independently.
func (m *MultiSource) SubscribeNewBlockEvent(ctx context.Context) (chan BlockEvent, error) {
	// One buffer slot per source: each can deposit an event without blocking the
	// others, beyond which the ctx-guarded send applies backpressure.
	out := make(chan BlockEvent, len(m.sources))

	var wg sync.WaitGroup
	for addr, src := range m.sources {
		wg.Add(1)
		go func(addr string, s blockSource) {
			defer wg.Done()
			m.subscribe(ctx, addr, s, out)
		}(addr, src)
	}

	// Close the fan-in channel only after every source goroutine has stopped, so
	// the consumer can detect end-of-stream and we never send on a closed channel.
	go func() {
		wg.Wait()
		close(out)
	}()

	return out, nil
}

// subscribe subscribes to a single source and forwards its heights into out,
// tagging each with the source's addr so GetSignedBlockFrom can fetch from the
// announcing peer, until the source's channel closes or ctx is canceled.
// Staying subscribed across transient endpoint errors is the underlying
// fetcher's responsibility.
func (m *MultiSource) subscribe(ctx context.Context, addr string, src blockSource, out chan<- BlockEvent) {
	sub, err := src.SubscribeNewBlockEvent(ctx)
	if err != nil {
		log.Warnw("multisource: subscribe failed", "source", addr, "err", err)
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-sub:
			if !ok {
				log.Debugw("multisource: source subscription closed", "source", addr)
				return
			}
			select {
			case out <- BlockEvent{Height: ev.Height, addr: addr}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// GetSignedBlockFrom fetches the block for the event from the single source
// that announced it — the fastest peer to notify this height, a fresh signal
// it's the most responsive peer. It does ONE thing and does not fall back to
// other sources: an error is returned as-is. Resilience is the fan-in's job —
// the same height is announced by other sources, and since a failed fetch
// stores nothing, the Listener re-fetches it from whichever source's duplicate
// event arrives next.
func (m *MultiSource) GetSignedBlockFrom(ctx context.Context, ev BlockEvent) (*SignedBlock, error) {
	src, ok := m.sources[ev.addr]
	if !ok {
		return nil, fmt.Errorf("multisource: unknown source %q for height %d", ev.addr, ev.Height)
	}
	blk, err := src.GetSignedBlock(ctx, ev.Height)
	if err != nil {
		return nil, fmt.Errorf("multisource: source %s: %w", ev.addr, err)
	}
	return blk, nil
}

// ChainID returns the chain ID from the first responsive source. All sources
// are expected to be on the same network; the Listener verifies the result
// against the expected chain ID.
func (m *MultiSource) ChainID(ctx context.Context) (string, error) {
	var errs error
	for addr, src := range m.sources {
		id, err := src.ChainID(ctx)
		if err == nil {
			return id, nil
		}
		errs = errors.Join(errs, fmt.Errorf("%s: %w", addr, err))
	}
	return "", fmt.Errorf("multisource: no source returned chain ID: %w", errs)
}

// IsSyncingFrom reports whether the source that announced the event is still
// catching up. Sync state is per-source: the same peer that announced and
// served the block answers whether it is a fresh head or a catch-up replay —
// another source being caught up says nothing about this block. Like
// GetSignedBlockFrom, it does not fall back to other sources: the Listener
// calls it before storing the height, so on error the duplicate announcement
// from another source retries the height whole.
func (m *MultiSource) IsSyncingFrom(ctx context.Context, ev BlockEvent) (bool, error) {
	src, ok := m.sources[ev.addr]
	if !ok {
		return false, fmt.Errorf("multisource: unknown source %q for height %d", ev.addr, ev.Height)
	}
	syncing, err := src.IsSyncing(ctx)
	if err != nil {
		return false, fmt.Errorf("multisource: source %s: %w", ev.addr, err)
	}
	return syncing, nil
}
