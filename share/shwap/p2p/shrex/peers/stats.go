package peers

import (
	"math"
	"time"

	"golang.org/x/time/rate"
)

// freshnessWindow is the idle duration after which a peer regains full exploration
// priority. A peer that has not been used for this long gets the maximum freshness
// bonus, nudging the selector to re-probe peers that went quiet.
const freshnessWindow = 30 * time.Second

// maxFreshnessBonus is the largest multiplicative boost freshness can add to a peer's
// score (i.e. an idle peer scores up to 1+maxFreshnessBonus times its quality score).
const maxFreshnessBonus = 0.5

// peerStats holds the per-peer quality and capacity state used by the scored pool to
// rank and pace peers. All fields are guarded by the owning pool's mutex.
type peerStats struct {
	// successEWMA is the exponentially-weighted moving average of request outcomes in
	// [0,1] (1 = success, 0 = failure). Initialized optimistically so new peers are
	// tried before their quality is known (ADR-014 optimistic initialization).
	successEWMA float64
	// latencyEWMA is the EWMA of successful-request latency in seconds. Only successes
	// update it; failures usually hit a full timeout and would poison the estimate.
	latencyEWMA float64
	// consecFails counts consecutive failures and drives the adaptive cooldown.
	consecFails int
	// totalSuccess and totalFailure are cumulative request-outcome counters, kept for
	// diagnostics (how much a peer has actually served) independently of the EWMAs.
	totalSuccess int
	totalFailure int
	// throughputEWMA is the EWMA of per-request throughput in bytes/second, measured on
	// successful requests that reported a payload size. Zero until the first measurement.
	throughputEWMA float64
	// totalBytes is the cumulative payload bytes served by this peer.
	totalBytes int64
	// inFlight is the number of outstanding requests currently assigned to this peer.
	inFlight int
	// lastUpdate is the time of the most recent handout/result, used for freshness decay.
	lastUpdate time.Time
	// limiter is the per-peer client-side token bucket used as a soft selection preference.
	limiter *rate.Limiter
}

func newPeerStats(params *Parameters, now time.Time) *peerStats {
	return &peerStats{
		successEWMA: 1, // optimistic: unproven peers look good enough to get tried
		latencyEWMA: 0, // optimistic: assume fast until measured
		lastUpdate:  now,
		limiter:     rate.NewLimiter(rate.Limit(params.PeerRateLimit), params.PeerRateBurst),
	}
}

// recordSuccess folds a successful request outcome into the EWMAs and resets the
// consecutive-failure counter. bytes is the payload size transferred (0 if unknown) and
// is used to maintain a per-peer throughput estimate.
func (s *peerStats) recordSuccess(latency time.Duration, bytes int64, params *Parameters, now time.Time) {
	a := params.EWMAAlpha
	s.successEWMA = a*1 + (1-a)*s.successEWMA
	s.latencyEWMA = a*latency.Seconds() + (1-a)*s.latencyEWMA
	if bytes > 0 {
		s.totalBytes += bytes
		if secs := latency.Seconds(); secs > 0 {
			tp := float64(bytes) / secs
			if s.throughputEWMA == 0 {
				// seed directly so the estimate is not anchored at zero
				s.throughputEWMA = tp
			} else {
				s.throughputEWMA = a*tp + (1-a)*s.throughputEWMA
			}
		}
	}
	s.consecFails = 0
	s.totalSuccess++
	s.lastUpdate = now
}

// recordFailure folds a failed request outcome into the success EWMA and escalates the
// consecutive-failure counter. Latency is intentionally not updated on failure.
func (s *peerStats) recordFailure(params *Parameters, now time.Time) {
	a := params.EWMAAlpha
	s.successEWMA = a*0 + (1-a)*s.successEWMA
	s.consecFails++
	s.totalFailure++
	s.lastUpdate = now
}

// quality returns the peer's load-independent quality score: higher is better. It
// combines reliability (successEWMA), speed (inverse latency) and a mild freshness
// bonus that re-elevates peers that have been idle.
func (s *peerStats) quality(now time.Time) float64 {
	latencyFactor := 1 / (1 + s.latencyEWMA)

	idle := now.Sub(s.lastUpdate)
	bonus := maxFreshnessBonus * math.Min(idle.Seconds()/freshnessWindow.Seconds(), 1)
	freshness := 1 + bonus

	return s.successEWMA * latencyFactor * freshness
}

// selectionScore is the value the pool maximizes when picking a peer. It applies a
// load penalty on top of quality so requests spread across peers, and a heavier
// penalty once a peer reaches its in-flight cap so it is only chosen when nothing
// better is eligible (spill, never block).
func (s *peerStats) selectionScore(now time.Time, params *Parameters) float64 {
	score := s.quality(now) / float64(1+s.inFlight)
	if s.inFlight >= params.InflightCap {
		score *= 0.01
	}
	// soft rate-limit preference: deprioritize peers we have recently hammered
	if s.limiter.Tokens() < 1 {
		score *= 0.5
	}
	return score
}

// adaptiveCooldown returns the cooldown duration for this peer given its current
// consecutive-failure count: base grows by CooldownFactor per failure up to MaxCooldown.
func (s *peerStats) adaptiveCooldown(params *Parameters) time.Duration {
	fails := s.consecFails
	if fails < 1 {
		fails = 1
	}
	// PeerCooldown * factor^(fails-1)
	mult := math.Pow(params.CooldownFactor, float64(fails-1))
	d := time.Duration(float64(params.PeerCooldown) * mult)
	if d > params.MaxCooldown || d <= 0 {
		d = params.MaxCooldown
	}
	return d
}
