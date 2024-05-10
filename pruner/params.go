package pruner

import (
	"fmt"
	"time"
)

type Option func(*Params)

type Params struct {
	// pruneCycle is the frequency at which the pruning Service
	// runs the ticker. If set to 0, the Service will not run.
	pruneCycle time.Duration
}

func (p *Params) Validate() error {
	if p.pruneCycle == time.Duration(0) {
		return fmt.Errorf("invalid GC cycle given, value should be positive and non-zero")
	}
	return nil
}

func DefaultParams() Params {
	return Params{
		pruneCycle: time.Minute * 5,
	}
}

// WithPruneCycle configures how often the pruning Service
// triggers a pruning cycle.
func WithPruneCycle(cycle time.Duration) Option {
	return func(p *Params) {
		p.pruneCycle = cycle
	}
}

// WithPrunerMetrics is a utility function to turn on pruner metrics and that is
// expected to be "invoked" by the fx lifecycle.
func WithPrunerMetrics(s *Service) error {
	return s.WithMetrics()
}
