package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Default probe cadence and circuit-breaker tuning. Exposed as variables
// (not constants) only to allow targeted overrides in tests; production
// code should treat them as immutable.
var (
	defaultTipProbeInterval      = 6 * time.Second
	defaultEarliestProbeInterval = 30 * time.Second
	defaultProbeTimeout          = 5 * time.Second
	defaultFailureThreshold      = 3
	defaultCooldown              = 30 * time.Second
)

// TrackerOption customises an EndpointTracker.
type TrackerOption func(*trackerParams)

type trackerParams struct {
	tipProbeInterval      time.Duration
	earliestProbeInterval time.Duration
	probeTimeout          time.Duration
	failureThreshold      int
	cooldown              time.Duration
	archivalHinted        bool
	priority              int
	now                   func() time.Time // for tests
}

func defaultTrackerParams() trackerParams {
	return trackerParams{
		tipProbeInterval:      defaultTipProbeInterval,
		earliestProbeInterval: defaultEarliestProbeInterval,
		probeTimeout:          defaultProbeTimeout,
		failureThreshold:      defaultFailureThreshold,
		cooldown:              defaultCooldown,
		now:                   time.Now,
	}
}

// WithTrackerArchivalHint declares an operator's expectation that the
// endpoint retains full history. It is a hint only — runtime probes are
// still authoritative.
func WithTrackerArchivalHint() TrackerOption {
	return func(p *trackerParams) { p.archivalHinted = true }
}

// WithTrackerPriority sets the routing priority. Lower is preferred; 0
// means primary. The tracker itself does not act on priority — the
// MultiBlockFetcher consults it.
func WithTrackerPriority(priority int) TrackerOption {
	return func(p *trackerParams) { p.priority = priority }
}

// WithTrackerProbeIntervals overrides probe cadence (intended for tests).
func WithTrackerProbeIntervals(tip, earliest time.Duration) TrackerOption {
	return func(p *trackerParams) {
		p.tipProbeInterval = tip
		p.earliestProbeInterval = earliest
	}
}

// WithTrackerCircuitBreaker overrides failure threshold and cooldown.
func WithTrackerCircuitBreaker(threshold int, cooldown time.Duration) TrackerOption {
	return func(p *trackerParams) {
		p.failureThreshold = threshold
		p.cooldown = cooldown
	}
}

// withTrackerClock is for tests only.
func withTrackerClock(now func() time.Time) TrackerOption {
	return func(p *trackerParams) { p.now = now }
}

// EndpointSnapshot is an immutable view of an EndpointTracker's state at a
// moment in time. Routing decisions in MultiBlockFetcher should be made
// off snapshots so the tracker's lock is held only briefly.
type EndpointSnapshot struct {
	Name            string
	Priority        int
	ArchivalHinted  bool
	ChainID         string
	EarliestHeight  int64
	LatestHeight    int64
	LatestBlockTime time.Time
	CatchingUp      bool
	Healthy         bool
	ConsecFailures  int
	LastProbe       time.Time
	LastErr         error
}

// Covers reports whether the endpoint is expected to have a block at the
// given height, based on the most recent probe.
func (s EndpointSnapshot) Covers(height int64) bool {
	if s.LatestHeight == 0 {
		// not yet probed — defer to caller policy. Treating as "covers" would
		// route requests to an unknown endpoint; treating as "doesn't cover"
		// would starve a fresh tracker. Caller picks; the routing layer
		// chooses to treat unknown as not-yet-eligible.
		return false
	}
	if height > s.LatestHeight {
		return false
	}
	if s.EarliestHeight > 0 && height < s.EarliestHeight {
		return false
	}
	return true
}

// EndpointTracker maintains health and coverage state for a single
// consensus endpoint. State is refreshed by a background probe loop and
// patched reactively from MultiBlockFetcher when point queries return
// NotFound.
type EndpointTracker struct {
	name    string
	fetcher Fetcher
	params  trackerParams

	mu              sync.RWMutex
	chainID         string
	earliestHeight  int64
	latestHeight    int64
	latestBlockTime time.Time
	catchingUp      bool
	consecFailures  int
	unhealthyUntil  time.Time
	lastProbe       time.Time
	lastErr         error

	// probeNow signals the loop to skip the current sleep and probe
	// immediately. Buffered with capacity 1 so MarkNotFound never blocks.
	probeNow chan struct{}

	cancel context.CancelFunc
	closed chan struct{}
}

// NewEndpointTracker constructs an EndpointTracker. It does NOT start the
// probe loop — call Run for that.
func NewEndpointTracker(name string, fetcher Fetcher, opts ...TrackerOption) *EndpointTracker {
	p := defaultTrackerParams()
	for _, opt := range opts {
		opt(&p)
	}
	return &EndpointTracker{
		name:     name,
		fetcher:  fetcher,
		params:   p,
		probeNow: make(chan struct{}, 1),
	}
}

// Name returns the tracker's identifier (used for logs and metrics).
func (t *EndpointTracker) Name() string { return t.name }

// Priority returns the routing priority configured for this tracker.
func (t *EndpointTracker) Priority() int { return t.params.priority }

// Run starts the background probe loop. It performs one synchronous probe
// before returning so callers can rely on the tracker having an initial
// snapshot once Run has returned. The loop runs until ctx is cancelled or
// Stop is called.
func (t *EndpointTracker) Run(ctx context.Context) error {
	t.mu.Lock()
	if t.cancel != nil {
		t.mu.Unlock()
		return fmt.Errorf("endpoint tracker %q: already running", t.name)
	}
	loopCtx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	t.closed = make(chan struct{})
	t.mu.Unlock()

	// initial synchronous probe so the tracker has state before Run returns
	t.probe(ctx, true)

	go t.loop(loopCtx)
	return nil
}

// Stop cancels the probe loop and waits for it to exit.
func (t *EndpointTracker) Stop(ctx context.Context) error {
	t.mu.Lock()
	cancel := t.cancel
	closed := t.closed
	t.cancel = nil
	t.mu.Unlock()

	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-closed:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Snapshot returns the tracker's current state. Cheap; safe to call
// repeatedly from routing hot paths.
func (t *EndpointTracker) Snapshot() EndpointSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return EndpointSnapshot{
		Name:            t.name,
		Priority:        t.params.priority,
		ArchivalHinted:  t.params.archivalHinted,
		ChainID:         t.chainID,
		EarliestHeight:  t.earliestHeight,
		LatestHeight:    t.latestHeight,
		LatestBlockTime: t.latestBlockTime,
		CatchingUp:      t.catchingUp,
		Healthy:         t.healthyLocked(),
		ConsecFailures:  t.consecFailures,
		LastProbe:       t.lastProbe,
		LastErr:         t.lastErr,
	}
}

// Healthy reports whether the tracker is currently eligible for routing.
// An endpoint becomes unhealthy after a configurable number of consecutive
// failures and re-enters rotation when the cooldown expires.
func (t *EndpointTracker) Healthy() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.healthyLocked()
}

func (t *EndpointTracker) healthyLocked() bool {
	if t.unhealthyUntil.IsZero() {
		return true
	}
	return !t.params.now().Before(t.unhealthyUntil)
}

// Covers is a convenience wrapper over Snapshot().Covers(height).
func (t *EndpointTracker) Covers(height int64) bool {
	return t.Snapshot().Covers(height)
}

// MarkSuccess resets the failure counter. Called by MultiBlockFetcher
// after any successful query.
func (t *EndpointTracker) MarkSuccess() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.consecFailures = 0
	t.unhealthyUntil = time.Time{}
	t.lastErr = nil
}

// MarkFailure records a generic failure. Once consecFailures reaches the
// configured threshold the tracker enters cooldown. Cancelled contexts
// are treated as caller-side and do not count against the budget.
func (t *EndpointTracker) MarkFailure(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		// Don't penalise an endpoint for the caller's own cancellation.
		// True deadline exceeded from the network would manifest as a
		// gRPC Unavailable; here we get the local context error.
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.consecFailures++
	t.lastErr = err
	if t.consecFailures >= t.params.failureThreshold {
		t.unhealthyUntil = t.params.now().Add(t.params.cooldown)
	}
}

// MarkNotFound is called when a point query returned a NotFound for a
// height that, according to the most recent snapshot, ought to have been
// covered. We tighten EarliestHeight conservatively (h+1) and trigger an
// out-of-band probe so the next routing decision uses fresh state.
//
// NotFound itself is not counted as a failure: it's an expected outcome
// when an endpoint has pruned a block.
func (t *EndpointTracker) MarkNotFound(height int64) {
	t.mu.Lock()
	if height >= t.earliestHeight && height <= t.latestHeight {
		t.earliestHeight = height + 1
	}
	t.mu.Unlock()
	t.kickProbe()
}

// IsNotFound reports whether err is a gRPC NotFound. MultiBlockFetcher
// uses this to decide whether to call MarkNotFound or MarkFailure.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if s, ok := status.FromError(err); ok {
		return s.Code() == codes.NotFound
	}
	return false
}

func (t *EndpointTracker) kickProbe() {
	select {
	case t.probeNow <- struct{}{}:
	default:
	}
}

func (t *EndpointTracker) loop(ctx context.Context) {
	defer close(t.closed)

	tipTimer := time.NewTimer(t.params.tipProbeInterval)
	earliestTimer := time.NewTimer(t.params.earliestProbeInterval)
	defer tipTimer.Stop()
	defer earliestTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tipTimer.C:
			t.probe(ctx, false)
			tipTimer.Reset(t.params.tipProbeInterval)
		case <-earliestTimer.C:
			t.probe(ctx, true)
			earliestTimer.Reset(t.params.earliestProbeInterval)
		case <-t.probeNow:
			t.probe(ctx, true)
			// drain timers so we don't double-probe right after
			if !tipTimer.Stop() {
				select {
				case <-tipTimer.C:
				default:
				}
			}
			tipTimer.Reset(t.params.tipProbeInterval)
		}
	}
}

// probe issues a Status RPC and merges the result into tracker state.
// fetchEarliest is currently informational — Status returns earliest
// unconditionally — but the flag is preserved so future implementations
// could batch or skip the cheaper-tip vs full-status distinction.
func (t *EndpointTracker) probe(ctx context.Context, fetchEarliest bool) {
	probeCtx, cancel := context.WithTimeout(ctx, t.params.probeTimeout)
	defer cancel()

	s, err := t.fetcher.Status(probeCtx)
	now := t.params.now()

	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastProbe = now

	if err != nil {
		// Don't count caller-cancellation as a failure.
		if !errors.Is(err, context.Canceled) {
			t.consecFailures++
			t.lastErr = err
			if t.consecFailures >= t.params.failureThreshold {
				t.unhealthyUntil = now.Add(t.params.cooldown)
			}
		}
		return
	}

	t.chainID = s.ChainID
	t.latestHeight = s.LatestHeight
	t.latestBlockTime = s.LatestBlockTime
	t.catchingUp = s.CatchingUp
	if fetchEarliest || t.earliestHeight == 0 {
		t.earliestHeight = s.EarliestHeight
	}
	t.consecFailures = 0
	t.unhealthyUntil = time.Time{}
	t.lastErr = nil
}
