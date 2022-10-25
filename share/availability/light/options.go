package light

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
	return func(la *ShareAvailability) {
		la.timeout = timeout
	}
}

// WithDefaultTimeout is a functional option that sets the availability
// to the default availability timeout defined in share/availability.go
func WithTimeoutDefault() Option {
	return func(la *ShareAvailability) {
		la.timeout = share.DefaultAvailabilityTimeout
	}
}

// WithSampleAmount is a functional option that ShareAvailability uses to set
// the SampleAmount
func WithSampleAmount(sampleAmount int) Option {
	return func(la *ShareAvailability) {
		la.sampleAmount = sampleAmount
	}
}

// WithDefaultSampleAmount is a functional option that sets
// the SampleAmount to the default value defined in share/availability/light/availability.go
func WithSampleAmountDefault() Option {
	return func(la *ShareAvailability) {
		la.sampleAmount = DefaultSampleAmount
	}
}
