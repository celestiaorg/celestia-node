package pruner

import (
	"time"
)

type Option func(*Params)

type Params struct {
	// gcCycle is the frequency at which the pruning Service
	// runs the ticker. If set to 0, the Service will not run.
	gcCycle time.Duration
}

func DefaultParams() Params {
	return Params{
		gcCycle: time.Hour,
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
