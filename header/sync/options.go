package sync

import (
	"fmt"
	"time"
)

type Options func(*Parameters)

// Parameters is the set of parameters that must be configured for the syncer.
type Parameters struct {
	// blockTime provides a reference point for the Syncer to determine
	// whether its subjective head is outdated
	BlockTime time.Duration
	// TrustingPeriod is period through which we can trust a header's validators set.
	//
	// Should be significantly less than the unbonding period (e.g. unbonding
	// period = 3 weeks, trusting period = 2 weeks).
	//
	// More specifically, trusting period + time needed to check headers + time
	// needed to report and punish misbehavior should be less than the unbonding
	// period.
	TrustingPeriod time.Duration
	// MaxRequestSizeNumber of headers that can be requested at once.
	MaxRequestSize uint64
}

// DefaultParameters returns the default params to configure the syncer.
func DefaultParameters() Parameters {
	return Parameters{
		BlockTime:      time.Second * 30,
		TrustingPeriod: 168 * time.Hour,
		MaxRequestSize: 512,
	}
}

func (p *Parameters) Validate() error {
	if p.BlockTime == 0 {
		return fmt.Errorf("invalid block time duration: %v", p.BlockTime)
	}
	if p.TrustingPeriod == 0 {
		return fmt.Errorf("invalid trusted time duration: %v", p.TrustingPeriod)
	}
	if p.MaxRequestSize == 0 {
		return fmt.Errorf("invalid max request size: %d", p.MaxRequestSize)
	}
	return nil
}

// WithBlockTime is a functional option that configures the
// `BlockTime` parameter.
func WithBlockTime(duration time.Duration) Options {
	return func(p *Parameters) {
		p.BlockTime = duration
	}
}

// WithTrustingPeriod is a functional option that configures the
// `TrustingPeriod` parameter.
func WithTrustingPeriod(duration time.Duration) Options {
	return func(p *Parameters) {
		p.TrustingPeriod = duration
	}
}

// WithMaxRequestSize is a functional option that configures the
// `MaxRequestSize` parameter.
func WithMaxRequestSize(amount uint64) Options {
	return func(p *Parameters) {
		p.MaxRequestSize = amount
	}
}
