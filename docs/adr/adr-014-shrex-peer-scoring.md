# ADR 014: Shrex Dynamic Peer Selection & Scoring

## Changelog

- 2026-07-08: initial draft (Tier 1 + Tier 2)

## Status

Proposed / In progress

## Context

The shrex peer manager (`share/shwap/p2p/shrex/peers`) hands out peers to the
shrex getter for every EDS/sample/row/namespace request. Today peer selection is
**blind round-robin** and the only feedback signal is a 3-valued enum
(`ResultNoop` / `ResultCooldownPeer` / `ResultBlacklistPeer`). This produces two
pathologies that are especially visible during **archival DAS catchup**, where a
node must fetch whole EDSs for historical heights and the archival pool is
populated only from discovery (no ShrexSub pools for old heights).

### Pathology 1 — no quality signal, no concentration

`pool.tryGet()` walks `peersList` in index order and returns the first `active`
peer. A 1-second-latency archival peer and a peer that will time out after 60s
are indistinguishable until *after* the request cost has been paid. With, say,
30 discovered archival peers where only a handful actually serve, round-robin
sprays ~90% of requests at peers that fail or crawl. Each failure can burn up to
`minRequestTimeout` (1 min, split across `minAttemptsCount = 3` attempts).
Catchup throughput is gated by the *slowest* peers, not the fastest.

### Pathology 2 — self-inflicted rate-limit thrash

The shrex **server** rate-limits per peer: per-IP `85 req/s` (burst `256`) and a
per-peer EDS stream cap of `16` (`share/shwap/p2p/shrex/limits.go`,
`rate_limit.go`). The **client** has zero self-pacing: all 16 DAS workers can
pile whole-EDS requests onto the single good peer simultaneously. This blows the
peer's per-peer EDS stream limit, the server resets the stream with
`ErrResourceExhausted`, the client maps that to `ResultCooldownPeer`, and — with
nothing else usable in the pool — blocks, waits out a flat 3s cooldown, and
repeats. The one peer that works gets punished for working.

### Framing

This is a **constrained multi-armed bandit / load-balancing** problem:

- Each peer is an *arm*; the *reward* is realized serving quality
  (success + low latency).
- We must **explore** (an unknown peer could be the good archival node) while
  **exploiting** (concentrate on known-good peers).
- Two hard **constraints** separate it from a textbook bandit: (a) per-peer
  capacity (rate + concurrency) means we cannot always pick the single best peer
  — we must spill; (b) rewards are non-stationary (peers churn, load changes),
  so estimates must decay.

## Decision

Evolve the peer manager from blind round-robin into a **quality-weighted,
capacity-aware selector**, delivered in tiers so each is independently shippable
and measurable. This ADR covers **Tier 1** and **Tier 2**; Tier 3 is recorded as
future work.

The `Peer()` / `DoneFunc` external contract is preserved. All new behaviour lives
inside the `peers` package (`pool.go`, `stats.go`, `timedqueue.go`,
`options.go`, `manager.go`).

### Signals

Collected cheaply at the two choke points that already exist — peer hand-out
(`Manager.Peer`) and completion (`DoneFunc`):

| Signal              | Source                                           | Cost         |
| ------------------- | ------------------------------------------------ | ------------ |
| success / failure   | the `result` already passed to `DoneFunc`        | free         |
| latency             | `time.Since(handoutTime)` measured in the manager| free         |
| in-flight count     | inc on hand-out, dec in `DoneFunc`               | 1 int / peer |
| consecutive failures| counter in stats                                 | 1 int / peer |

Note: latency is measured **inside the manager** by timing the interval between
handing a peer out and its `DoneFunc` being called. This avoids any change to
the getter hot path. Response **byte size** (for true throughput) is deliberately
*not* plumbed in this ADR — within a single availability window block sizes are
similar, so latency of a successful fetch is a good enough quality proxy.
Byte-accurate throughput is a Tier 3 refinement.

### Tier 1 — capacity awareness (no learning)

Attacks the thrash and wasted-timeout costs without any scoring model:

1. **Per-peer in-flight cap.** Track outstanding requests per peer; a peer at its
   cap is skipped during selection. Default cap is set safely below the server's
   per-peer EDS stream limit of 16. This alone stops the dogpile that triggers
   `ErrResourceExhausted`: overflow spills to other peers or blocks instead of
   punishing the good peer.
2. **Per-peer rate limiter (token bucket).** A `golang.org/x/time/rate.Limiter`
   per peer, sized *below* the server's 85 req/s, so the client self-paces rather
   than discovering the limit by getting reset. A peer with no token available is
   treated as temporarily ineligible (not a failure).
3. **Adaptive cooldown.** Replaces the flat 3s. Cooldown grows with consecutive
   failures and is keyed on the error class:
   `cooldown = min(base * factor^(consecFailures-1), max)` with jitter.
   `ErrResourceExhausted` uses a short base (peer is fine, just busy);
   deadline / not-found use a longer base. Good peers with a single blip barely
   cool; chronically bad peers fall out of rotation on their own. Implemented by
   generalising `timedQueue` to a **per-item TTL**.
4. **Short probe timeout for unproven peers.** *(Deferred — see below.)* The idea
   is that the first attempt against a never-seen peer uses a short timeout so a
   dead peer costs seconds, not a full minute. A *blanket* short first attempt is
   unsafe: a legitimately slow whole-EDS fetch on a good peer (the getter's own
   `defaultMinRequestTimeout` is 1 min for a 256-size block) would be cut off and
   the good peer wrongly cooled. Doing it safely requires the peer manager to
   signal the selected peer's *confidence* (proven vs unproven) to the getter so
   the short timeout applies only to unproven peers. That plumbing is deferred to
   a follow-up; the adaptive cooldown already shortens the penalty for a first
   failure to the base (3s), so an unproven dead peer is shed relatively cheaply
   even without it.

### Tier 2 — quality scoring + Power-of-Two-Choices

Adds per-peer quality estimation and changes the selection primitive.

**Per-peer score** (all EWMA-decayed, a few floats per peer):

```
score = successEWMA * latencyFactor * freshness
latencyFactor = 1 / (1 + latencyEWMA_seconds)
```

- `successEWMA`: EWMA of 1.0 (success) / 0.0 (failure).
- `latencyEWMA`: EWMA of successful-request latency; via `latencyFactor` a faster
  peer scores higher.
- `freshness`: mild time decay so a peer we have not used recently regains
  exploration priority.
- **Optimistic initialization**: an unknown peer is given a high default score so
  it is tried within the first few selections, then converges to its true value.
  This is how the good archival peer is discovered among many without a separate
  exploration schedule.

**Selection — weighted Power-of-Two-Choices (P2C).** Instead of scanning for the
global best (which herds every worker onto one peer and re-triggers Pathology 2),
sample `K` (default 2) random *eligible* peers and pick the higher score:

```
eligible(peer) := active AND inFlight < cap AND rateLimiter.allow
sample K eligible peers uniformly at random
return argmax(score) among the sample
```

P2C gives near-optimal max-load with O(K) selection, and because two workers
rarely draw the same pair it naturally fans traffic across the top few peers —
staying under each peer's rate cap while still concentrating on good peers. It
also removes the O(n) round-robin scan over stale entries.

**Feedback.** `DoneFunc` invocations update `successEWMA`, `latencyEWMA`,
`consecutiveFailures`, and decrement in-flight. Scoring is latched on the first
result per hand-out (the getter may call `setStatus` twice on the verify-failure
path); the cooldown/blacklist *action* still runs per call as today.

### Tier 3 — future work (not in this ADR)

- Byte-accurate throughput scoring (plumb response size through `DoneFunc`).
- Thompson sampling over a throughput posterior with explicit per-peer capacity
  constraints, if measurements show P2C + optimistic-init under-explores.
- Getter-side short probe timeout for unproven peers, gated on a peer-confidence
  signal plumbed from the peer manager (see Tier 1 item 4).
- Error-class-keyed cooldown base (short for `ErrResourceExhausted`, long for
  deadline/not-found). Requires the getter to pass the error class through
  `DoneFunc`; today all cooldown-worthy errors collapse to `ResultCooldownPeer`,
  so the adaptive cooldown escalates on consecutive-failure count only.

## Consequences

### Positive

- Catchup concentrates on fast peers instead of being gated by the slowest.
- The single-good-peer case converges to "drive that peer at its capacity, don't
  thrash it," instead of the cooldown loop.
- `ErrResourceExhausted` from self-inflicted overload should drop toward zero.
- Selection becomes O(K) instead of O(n) over the peer list.

### Negative / risks

- More per-peer state (a small stats struct + a rate limiter per peer). Bounded
  by pool size; cleaned up with the peer.
- New parameters to tune (cap, cooldown base/factor/max, EWMA half-life, K, rate).
  Defaults are chosen conservatively from the known server limits; tuning is
  expected via the simulation harness and testnet metrics.
- Optimistic init means brief probing of bad peers; mitigated by the short probe
  timeout and fast cooldown escalation.

### Neutral

- Blacklisting behaviour (`EnableBlackListing`, off by default) is unchanged.
- The full/archival peer-manager split is unchanged; both gain the same scored
  pool. Archival benefits most because its pool is discovery-only and noisiest.

## Evaluation

New metrics (extending `peers/metrics.go`): selection concentration
(entropy / Gini over peers), client-observed `ErrResourceExhausted` rate,
time-to-first-good-peer after start, and per-peer score/latency distributions.
Best validated offline with a replay/simulation harness that feeds a synthetic
pool (1 fast archival peer + N stale) through the manager and compares catchup
rate under round-robin vs Tier 1 vs Tier 2 before touching a live node.

## References

- `share/shwap/p2p/shrex/peers/` — peer manager, pool, timed queue
- `share/shwap/p2p/shrex/limits.go`, `rate_limit.go` — server-side limits this
  design mirrors on the client
- `share/shwap/p2p/shrex/shrex_getter/shrex.go` — getter call sites / result mapping
- Mitzenmacher, "The Power of Two Choices in Randomized Load Balancing"
- ADR 012 — DASer parallelization (the consumer whose throughput this unblocks)
