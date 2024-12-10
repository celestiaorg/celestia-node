package full

type params struct {
	archival bool
}

// Option is a function that configures light availability Parameters
type Option func(*params)

// DefaultParameters returns the default Parameters' configuration values
// for the light availability implementation
func defaultParams() *params {
	return &params{
		archival: false,
	}
}

// WithArchivalMode is a functional option to tell the full availability
// implementation that the node wants to sync *all* blocks, not just those
// within the sampling window.
func WithArchivalMode() Option {
	return func(p *params) {
		p.archival = true
	}
}
