package light

import (
	"fmt"
)

// DefaultSampleAmount specifies the minimum required amount of samples a light node must perform
// before declaring that a block is available
var (
	DefaultSampleAmount uint = 16
)

// Parameters is the set of Parameters that must be configured for the light
// availability implementation
type Parameters struct {
	SampleAmount uint // The minimum required amount of samples to perform
}

// Option is a function that configures light availability Parameters
type Option func(*Parameters)

// DefaultParameters returns the default Parameters' configuration values
// for the light availability implementation
func DefaultParameters() *Parameters {
	return &Parameters{
		SampleAmount: DefaultSampleAmount,
	}
}

// Validate validates the values in Parameters
func (p *Parameters) Validate() error {
	if p.SampleAmount <= 0 {
		return fmt.Errorf(
			"light availability: invalid option: value %s was %s, where it should be %s",
			"SampleAmount",
			"<= 0", // current value
			">= 0", // what the value should be
		)
	}

	return nil
}

// WithSampleAmount is a functional option that the Availability interface
// implementers use to set the SampleAmount configuration param
func WithSampleAmount(sampleAmount uint) Option {
	return func(p *Parameters) {
		p.SampleAmount = sampleAmount
	}
}
