package pruner

import (
	"time"
)

type Option func(*Params)

type Params struct {
	// gcCycle is the frequency at which the pruning Service
	// runs the ticker. If set to 0, the Service will not run.
	gcCycle time.Duration
	// maxPruneablePerGC sets an upper limit to how many blocks
	// can be pruned during a GC cycle.
	maxPruneablePerGC uint64
}

func DefaultParams() Params {
	return Params{
		gcCycle:           time.Minute * 5,
		maxPruneablePerGC: 1024,
	}
}

// WithGCCycle configures how often the pruning Service
// triggers a pruning cycle.
func WithGCCycle(cycle time.Duration) Option {
	return func(p *Params) {
		p.gcCycle = cycle
	}
}

// WithDisabledGC disables the pruning Service's pruning
// routine.
func WithDisabledGC() Option {
	return func(p *Params) {
		p.gcCycle = time.Duration(0)
	}
}

// WithPrunerMetrics is a utility function to turn on pruner metrics and that is
// expected to be "invoked" by the fx lifecycle.
func WithPrunerMetrics(s *Service) error {
	return s.WithMetrics()
}

// WithMaxPruneablePerGC sets the upper limit for how many blocks can
// be pruned per GC cycle.
func WithMaxPruneablePerGC(maxPruneable uint64) Option {
	return func(p *Params) {
		p.maxPruneablePerGC = maxPruneable
	}
}
