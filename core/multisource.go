package core

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc"
)

// taggedSource pairs a source with its endpoint address (conn.Target)
type taggedSource struct {
	fetcher Fetcher
	addr    string
}

// MultiSource fans several core endpoints into one new-block stream. Each
// endpoint stays subscribed independently (the underlying BlockFetcher already
// resubscribes forever on its own), so a single failing endpoint cannot stall
// the others. Duplicate heights across sources are expected and must be deduplicated by
// the consumer.
type MultiSource struct {
	sources []taggedSource
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
	return &MultiSource{sources: sources}
}

// Verify checks every source against the expected network. A source that is
// definitively on the wrong chain is pruned (and logged with its address); a
// source that could not be reached is kept, so it may join once it comes up — a
// wrong-chain block from it later is caught by the Listener's per-block backstop.
// It errors if no source could be confirmed on the expected network, so a fully
// misconfigured or unreachable set refuses to start. The expected chain ID must
// be set: a node must know which network it serves.
//
// Verify mutates m.sources and is safe only because Listener.Start calls it
// synchronously before SubscribeNewBlockEvent spawns goroutines and before
// ChainID/IsSyncing run. Calling it concurrently with those would race.
func (m *MultiSource) Verify(ctx context.Context, expected string) error {
	if expected == "" {
		return fmt.Errorf("multisource: expected chain ID must be configured")
	}

	// Query every source's chain ID concurrently so a slow or unreachable
	// endpoint doesn't serialize startup behind it: total latency is the slowest
	// source rather than the sum. Each goroutine writes its own results slot, so
	// the wg barrier alone orders the writes before the read below — no mutex.
	ids := make([]string, len(m.sources))
	errs := make([]error, len(m.sources))
	var wg sync.WaitGroup
	for i, src := range m.sources {
		wg.Add(1)
		go func(i int, src taggedSource) {
			defer wg.Done()
			ids[i], errs[i] = src.fetcher.ChainID(ctx)
		}(i, src)
	}
	wg.Wait()

	// Assemble the surviving set sequentially: deterministic and free of shared
	// mutation now that all goroutines have returned.
	kept := make([]taggedSource, 0, len(m.sources))
	confirmed := 0
	for i, src := range m.sources {
		switch {
		case errs[i] != nil:
			log.Warnw("multisource: could not verify chain ID, keeping source",
				"source", src.addr, "err", errs[i])
			kept = append(kept, src)
		case ids[i] != expected:
			log.Errorw("multisource: dropping endpoint on wrong network",
				"source", src.addr, "expected", expected, "received", ids[i])
		default:
			confirmed++
			kept = append(kept, src)
		}
	}
	m.sources = kept
	if confirmed == 0 {
		return fmt.Errorf("multisource: no source confirmed on expected network %q", expected)
	}
	return nil
}

// SubscribeNewBlockEvent fans every source's new-block subscription into one
// channel, closed once all source goroutines exit (i.e. ctx is canceled).
func (m *MultiSource) SubscribeNewBlockEvent(ctx context.Context) (chan SignedBlock, error) {
	// One buffer slot per source: each can deposit a block without blocking the
	// others, beyond which the ctx-guarded send applies backpressure.
	out := make(chan SignedBlock, len(m.sources))

	var wg sync.WaitGroup
	for _, src := range m.sources {
		wg.Add(1)
		go func(s taggedSource) {
			defer wg.Done()
			m.subscribe(ctx, s, out)
		}(src)
	}

	// Close the fan-in channel only after every source goroutine has stopped, so
	// the consumer can detect end-of-stream and we never send on a closed channel.
	go func() {
		wg.Wait()
		close(out)
	}()

	return out, nil
}

// subscribe subscribes to a single source and forwards its blocks into out until
// the source's channel closes or ctx is canceled. Staying subscribed across
// transient endpoint errors is the underlying fetcher's responsibility.
func (m *MultiSource) subscribe(ctx context.Context, src taggedSource, out chan<- SignedBlock) {
	sub, err := src.fetcher.SubscribeNewBlockEvent(ctx)
	if err != nil {
		log.Warnw("multisource: subscribe failed", "source", src.addr, "err", err)
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case blk, ok := <-sub:
			if !ok {
				log.Debugw("multisource: source subscription closed", "source", src.addr)
				return
			}
			select {
			case out <- blk:
			case <-ctx.Done():
				return
			}
		}
	}
}

// ChainID returns the chain ID from the first responsive source. All sources
// are expected to be on the same network; the Listener verifies the result
// against the expected chain ID.
func (m *MultiSource) ChainID(ctx context.Context) (string, error) {
	var errs error
	for _, src := range m.sources {
		id, err := src.fetcher.ChainID(ctx)
		if err == nil {
			return id, nil
		}
		errs = errors.Join(errs, fmt.Errorf("%s: %w", src.addr, err))
	}
	return "", fmt.Errorf("multisource: no source returned chain ID: %w", errs)
}

// IsSyncing reports whether core should be treated as still catching up. All
// sources are queried concurrently and it returns false (not syncing) as soon
// as any source reports it is synced — so the Listener broadcasts to the
// network whenever at least one source is caught up — canceling the rest. If
// every reachable source reports syncing it returns true; only if no source
// could be reached does it return an error.
func (m *MultiSource) IsSyncing(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		syncing bool
		err     error
	}
	ch := make(chan result, len(m.sources))
	for _, src := range m.sources {
		go func(s taggedSource) {
			syncing, err := s.fetcher.IsSyncing(ctx)
			if err != nil {
				err = fmt.Errorf("%s: %w", s.addr, err)
			}
			select {
			case ch <- result{syncing, err}:
			case <-ctx.Done():
			}
		}(src)
	}

	var (
		errs    error
		anyResp bool
	)
	for range m.sources {
		select {
		case r := <-ch:
			if r.err != nil {
				errs = errors.Join(errs, r.err)
				continue
			}
			anyResp = true
			if !r.syncing {
				return false, nil // at least one source is caught up
			}
		case <-ctx.Done():
			return true, ctx.Err()
		}
	}
	if !anyResp {
		return true, fmt.Errorf("multisource: no source returned sync state: %w", errs)
	}
	return true, nil // every reachable source is still syncing
}
