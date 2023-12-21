package pruner

import "time"

type Option func(*Params)

type Params struct {
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
