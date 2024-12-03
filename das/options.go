package das

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidOption is an error that is returned by Parameters.Validate
// when supplied with invalid values.
// This error will also be returned by NewDASer if supplied with an invalid option
var ErrInvalidOption = errors.New("das: invalid option")

// errInvalidOptionValue is a utility function to dedup code for error-returning
// when dealing with invalid parameter values
func errInvalidOptionValue(optionName, value string) error {
	return fmt.Errorf("%w: value %s cannot be %s", ErrInvalidOption, optionName, value)
}

// Option is the functional option that is applied to the daser instance
// to configure DASing parameters (the Parameters struct)
type Option func(*DASer)

// Parameters is the set of parameters that must be configured for the daser
type Parameters struct {
	//  SamplingRange is the maximum amount of headers processed in one job.
	SamplingRange uint64

	// ConcurrencyLimit defines the maximum amount of sampling workers running in parallel.
	ConcurrencyLimit int

	// BackgroundStoreInterval is the period of time for background checkpointStore to perform a
	// checkpoint backup.
	BackgroundStoreInterval time.Duration

	// SampleFrom is the height sampling will start from if no previous checkpoint was saved
	SampleFrom uint64

	// SampleTimeout is a maximum amount time sampling of single block may take until it will be
	// canceled. High ConcurrencyLimit value may increase sampling time due to node resources being
	// divided between parallel workers. SampleTimeout should be adjusted proportionally to
	// ConcurrencyLimit.
	SampleTimeout time.Duration
}

// DefaultParameters returns the default configuration values for the daser parameters
func DefaultParameters() Parameters {
	// TODO(@derrandz): parameters needs performance testing on real network to define optimal values
	// (#1261)
	concurrencyLimit := 16
	return Parameters{
		SamplingRange:           100,
		ConcurrencyLimit:        concurrencyLimit,
		BackgroundStoreInterval: 10 * time.Minute,
		SampleFrom:              1,
		// SampleTimeout = approximate block time (with a bit of wiggle room) * max amount of catchup
		// workers
		SampleTimeout: 15 * time.Second * time.Duration(concurrencyLimit),
	}
}

// Validate validates the values in Parameters
//
//	All parameters must be positive and non-zero, except:
//		BackgroundStoreInterval = 0 disables background storer,
//		PriorityQueueSize = 0 disables prioritization of recently produced blocks for sampling
func (p *Parameters) Validate() error {
	// SamplingRange = 0 will cause the jobs' queue to be empty
	// Therefore no sampling jobs will be reserved and more importantly the DASer will break
	if p.SamplingRange <= 0 {
		return errInvalidOptionValue(
			"SamplingRange",
			"negative or 0",
		)
	}

	// ConcurrencyLimit = 0 will cause the number of workers to be 0 and
	// Thus no threads will be assigned to the waiting jobs therefore breaking the DASer
	if p.ConcurrencyLimit <= 0 {
		return errInvalidOptionValue(
			"ConcurrencyLimit",
			"negative or 0",
		)
	}

	// SampleFrom = 0 would tell the DASer to start sampling from block height 0
	// which does not exist therefore breaking the DASer.
	if p.SampleFrom <= 0 {
		return errInvalidOptionValue(
			"SampleFrom",
			"negative or 0",
		)
	}

	// SampleTimeout = 0 would fail every sample operation with timeout error
	if p.SampleTimeout <= 0 {
		return errInvalidOptionValue(
			"SampleTimeout",
			"negative or 0",
		)
	}

	return nil
}

// WithSamplingRange is a functional option to configure the daser's `SamplingRange` parameter
//
//	Usage:
//	```
//		WithSamplingRange(10)(daser)
//	```
//
// or
//
//	```
//		option := WithSamplingRange(10)
//		// shenanigans to create daser
//		option(daser)
//
// ```
func WithSamplingRange(samplingRange uint64) Option {
	return func(d *DASer) {
		d.params.SamplingRange = samplingRange
	}
}

// WithConcurrencyLimit is a functional option to configure the daser's `ConcurrencyLimit` parameter
// Refer to WithSamplingRange documentation to see an example of how to use this
func WithConcurrencyLimit(concurrencyLimit int) Option {
	return func(d *DASer) {
		d.params.ConcurrencyLimit = concurrencyLimit
	}
}

// WithBackgroundStoreInterval is a functional option to configure the daser's
// `backgroundStoreInterval` parameter Refer to WithSamplingRange documentation to see an example
// of how to use this
func WithBackgroundStoreInterval(backgroundStoreInterval time.Duration) Option {
	return func(d *DASer) {
		d.params.BackgroundStoreInterval = backgroundStoreInterval
	}
}

// WithSampleFrom is a functional option to configure the daser's `SampleFrom` parameter
// Refer to WithSamplingRange documentation to see an example of how to use this
func WithSampleFrom(sampleFrom uint64) Option {
	return func(d *DASer) {
		d.params.SampleFrom = sampleFrom
	}
}

// WithSampleTimeout is a functional option to configure the daser's `SampleTimeout` parameter
// Refer to WithSamplingRange documentation to see an example of how to use this
func WithSampleTimeout(sampleTimeout time.Duration) Option {
	return func(d *DASer) {
		d.params.SampleTimeout = sampleTimeout
	}
}
