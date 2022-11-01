package light

import (
	"fmt"
	"time"

	"github.com/celestiaorg/celestia-node/share"
)

// ErrInvalidOption is an error that is returned by Parameters.Validate
// when supplied with invalid values.
// This error will also be returned by NewShareAvailability if supplied with an invalid option
var ErrInvalidOption = fmt.Errorf("light availability: invalid option")

// errInvalidOptionValue is a utility function to dedup code for error-returning
// when dealing with invalid parameter values.
func errInvalidOptionValue(optionName string, value string) error {
	return fmt.Errorf("%w: value %s cannot be %s", ErrInvalidOption, optionName, value)
}

// Option is the type for the functional options that ShareAvailability uses to set parameters
type Option func(*ShareAvailability)

// Parameters is the set of parameters that must be configured for the light availability implementation
type Parameters struct {
	Timeout      time.Duration //
	SampleAmount int           // The minimum required amount of samples to perform
}

// DefaultParameters returns the default parameters' configuration values for the light availability implementation
func DefaultParameters() Parameters {
	return Parameters{
		Timeout:      share.DefaultAvailabilityTimeout,
		SampleAmount: DefaultSampleAmount,
	}
}

// Validate validates the values in Parameters
func (p *Parameters) Validate() error {
	if p.Timeout <= 0 {
		return errInvalidOptionValue(
			"Timeout",
			"native or 0",
		)
	}

	if p.SampleAmount <= 0 {
		return errInvalidOptionValue(
			"SampleAmount",
			"negative or 0",
		)
	}

	return nil
}

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
		la.params.Timeout = timeout
	}
}

// WithSampleAmount is a functional option that ShareAvailability uses to set
// the SampleAmount
func WithSampleAmount(sampleAmount int) Option {
	return func(la *ShareAvailability) {
		la.params.SampleAmount = sampleAmount
	}
}
