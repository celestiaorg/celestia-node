package pruner

import (
	"fmt"
	"time"
)

type Option func(*Params)

type Params struct {
	// gcCycle is the frequency at which the pruning Service
	// runs the ticker. If set to 0, the Service will not run.
	gcCycle time.Duration
}

func (p *Params) Validate() error {
	if p.gcCycle == time.Duration(0) {
		return fmt.Errorf("invalid GC cycle given, value should be positive and non-zero")
	}
	return nil
}

func DefaultParams() Params {
	return Params{
		gcCycle: time.Minute * 5,
	}
}

// WithGCCycle configures how often the pruning Service
// triggers a pruning cycle.
func WithGCCycle(cycle time.Duration) Option {
	return func(p *Params) {
		p.gcCycle = cycle
	}
}

// WithPrunerMetrics is a utility function to turn on pruner metrics and that is
// expected to be "invoked" by the fx lifecycle.
func WithPrunerMetrics(s *Service) error {
	return s.WithMetrics()
}
