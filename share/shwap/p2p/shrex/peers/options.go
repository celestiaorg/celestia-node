package peers

import (
	"fmt"
	"time"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
)

type Parameters struct {
	// PoolValidationTimeout is the timeout used for validating incoming datahashes. Pools that have
	// been created for datahashes from shrexsub that do not see this hash from headersub after this
	// timeout will be garbage collected.
	PoolValidationTimeout time.Duration

	// PeerCooldown is the base time a peer is put on cooldown after a ResultCooldownPeer. It is the
	// starting point for the adaptive (escalating) cooldown; see CooldownFactor and MaxCooldown.
	PeerCooldown time.Duration

	// GcInterval is the interval at which the manager will garbage collect unvalidated pools.
	GcInterval time.Duration

	// EnableBlackListing turns on blacklisting for misbehaved peers
	EnableBlackListing bool

	// --- Dynamic peer selection (ADR-014) ---

	// CooldownFactor is the multiplier applied per consecutive failure to grow the cooldown from
	// PeerCooldown up to MaxCooldown: cooldown = PeerCooldown * CooldownFactor^(consecFails-1).
	CooldownFactor float64

	// MaxCooldown caps the adaptive cooldown so a flapping peer is retried at least this often.
	MaxCooldown time.Duration

	// EWMAAlpha is the smoothing factor in (0,1] for the success- and latency-EWMA per peer. Higher
	// values weight recent observations more heavily.
	EWMAAlpha float64

	// InflightCap is the soft per-peer concurrency cap. Selection strongly deprioritizes peers at or
	// above this many in-flight requests, spilling load to other peers, but never blocks below the
	// server's per-peer stream limit when a peer is the only option.
	InflightCap int

	// P2CSampleSize is the number of random candidates drawn for Power-of-Two-Choices selection. The
	// highest-scoring candidate of the sample is chosen. 1 disables P2C (pure greedy).
	P2CSampleSize int

	// PeerRateLimit is the per-peer client-side request rate (req/s) used as a soft selection
	// preference. Sized below the shrex server's per-IP limit so the client self-paces.
	PeerRateLimit float64

	// PeerRateBurst is the token-bucket burst size for the per-peer rate limiter.
	PeerRateBurst int

	// ThroughputWeight controls how strongly observed download throughput (bytes/sec)
	// influences peer selection, in [0,1]. 0 disables throughput weighting entirely
	// (selection falls back to success/latency only); higher values bias selection more
	// toward faster peers. The resulting multiplier stays within [1-ThroughputWeight,
	// 1+ThroughputWeight], so it re-ranks peers without ever excluding a peer outright.
	ThroughputWeight float64

	// ThroughputRefBytesPerSec is the reference throughput at which a peer is treated as
	// "average" (multiplier ~1). Peers faster than this are boosted, slower ones demoted.
	// Only used when ThroughputWeight > 0.
	ThroughputRefBytesPerSec float64
}

type Option func(*Manager) error

// Validate validates the values in Parameters
func (p *Parameters) Validate() error {
	if p.PoolValidationTimeout <= 0 {
		return fmt.Errorf("peer-manager: validation timeout must be positive")
	}

	if p.PeerCooldown <= 0 {
		return fmt.Errorf("peer-manager: peer cooldown must be positive")
	}

	if p.GcInterval <= 0 {
		return fmt.Errorf("peer-manager: garbage collection interval must be positive")
	}

	if p.CooldownFactor < 1 {
		return fmt.Errorf("peer-manager: cooldown factor must be >= 1")
	}

	if p.MaxCooldown < p.PeerCooldown {
		return fmt.Errorf("peer-manager: max cooldown must be >= peer cooldown")
	}

	if p.EWMAAlpha <= 0 || p.EWMAAlpha > 1 {
		return fmt.Errorf("peer-manager: EWMA alpha must be in (0, 1]")
	}

	if p.InflightCap <= 0 {
		return fmt.Errorf("peer-manager: inflight cap must be positive")
	}

	if p.P2CSampleSize <= 0 {
		return fmt.Errorf("peer-manager: P2C sample size must be positive")
	}

	if p.PeerRateLimit <= 0 {
		return fmt.Errorf("peer-manager: peer rate limit must be positive")
	}

	if p.PeerRateBurst <= 0 {
		return fmt.Errorf("peer-manager: peer rate burst must be positive")
	}

	// ThroughputWeight is optional; the zero value disables throughput weighting and keeps
	// this non-breaking for configs written before the field existed.
	if p.ThroughputWeight < 0 || p.ThroughputWeight > 1 {
		return fmt.Errorf("peer-manager: throughput weight must be in [0, 1]")
	}

	if p.ThroughputWeight > 0 && p.ThroughputRefBytesPerSec <= 0 {
		return fmt.Errorf("peer-manager: throughput reference must be positive when throughput weight > 0")
	}

	return nil
}

// DefaultParameters returns the default configuration values for the peer manager parameters
func DefaultParameters() *Parameters {
	return &Parameters{
		// PoolValidationTimeout's default value is based on the default daser sampling timeout of 1 minute.
		// If a received datahash has not tried to be sampled within these two minutes, the pool will be
		// removed.
		PoolValidationTimeout: 2 * time.Minute,
		// PeerCooldown's default value is based on initial network tests that showed a ~3.5 second
		// sync time for large blocks. This value gives our (discovery) peers enough time to sync
		// the new block before we ask them again.
		PeerCooldown: 3 * time.Second,
		GcInterval:   time.Second * 30,
		// blacklisting is off by default //TODO(@walldiss): enable blacklisting once all related issues
		// are resolved
		EnableBlackListing: false,

		// Dynamic peer selection defaults (ADR-014). Chosen conservatively from the shrex server's
		// per-peer limits (16 EDS streams, 85 req/s per IP) so the client self-paces below them.
		CooldownFactor: 2,
		MaxCooldown:    time.Minute,
		EWMAAlpha:      0.3,
		InflightCap:    12,
		P2CSampleSize:  2,
		PeerRateLimit:  60,
		PeerRateBurst:  64,
		// Throughput weighting: bias selection toward faster peers. Moderate weight so a
		// slow peer is demoted but not starved; reference set near the middle of observed
		// archival throughputs (~20 MiB/s). Set ThroughputWeight to 0 to A/B the old
		// success/latency-only behavior.
		ThroughputWeight:         0.5,
		ThroughputRefBytesPerSec: 20 << 20, // 20 MiB/s
	}
}

// WithShrexSubPools passes a shrexsub and headersub instance to be used to populate and validate
// pools from shrexsub notifications.
func WithShrexSubPools(shrexSub *shrexsub.PubSub, headerSub libhead.Subscriber[*header.ExtendedHeader]) Option {
	return func(m *Manager) error {
		m.shrexSub = shrexSub
		m.headerSub = headerSub
		return nil
	}
}

// WithMetrics turns on metric collection in peer manager.
func (m *Manager) WithMetrics() error {
	metrics, err := initMetrics(m)
	if err != nil {
		return fmt.Errorf("peer-manager: init metrics: %w", err)
	}
	m.metrics = metrics
	return nil
}
