package full

import (
	"time"

	"github.com/celestiaorg/celestia-node/share"
)

// Option is the type for the functional options that ShareAvailability uses to set parameters
type Option func(*ShareAvailability)

// WithTimeout is the functional option to set the availability timeout
// To be used with the construction
// example:
//
// NewShareAvailability(
//
//	bServ,
//	disc,
//	WithTimeout(10 * time.Minute),
//
// )
func WithTimeout(timeout time.Duration) Option {
	return func(fa *ShareAvailability) {
		fa.timeout = timeout
	}
}

// WithDefaultTimeout is a functional option that sets the availability
// to the default availability timeout defined in share/availability.go
func WithTimeoutDefault() Option {
	return func(fa *ShareAvailability) {
		fa.timeout = share.DefaultAvailabilityTimeout
	}
}
