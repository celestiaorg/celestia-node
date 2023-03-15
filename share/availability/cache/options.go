package cache

import (
	"fmt"
)

const (
	// DefaultWriteBatchSize defines the size of the batched header write.
	// Headers are written in batches not to thrash the underlying Datastore with writes.
	// TODO(@Wondertan, @renaynay): Those values must be configurable
	// and proper defaults should be set for specific node type. (#709)
	DefaultWriteBatchSize          = 2048
	DefaultCacheAvailabilityPrefix = "sampling_result"
)

// Parameters is the set of Parameters that must be configured for cache
// availability implementation
type Parameters struct {
	// WriteBatchSize defines the size of the batched header write.
	WriteBatchSize uint
	// The string prefix to use as a key for the datastore
	CacheAvailabilityPrefix string
}

// Option is a function that configures cache availability Parameters
type Option func(*Parameters)

// DefaultParameters returns the default Parameters' configuration values
// for the light availability implementation
func DefaultParameters() Parameters {
	return Parameters{
		WriteBatchSize:          DefaultWriteBatchSize,
		CacheAvailabilityPrefix: DefaultCacheAvailabilityPrefix,
	}
}

// Validate validates the values in Parameters
func (ca *Parameters) Validate() error {
	if ca.WriteBatchSize <= 0 {
		return fmt.Errorf(
			"cache availability: invalid option: value %s was %s, where it should be %s",
			"DefaultWriteBatchSize",
			"<= 0", // current value
			">= 0", // what the valueshould be
		)
	}

	if ca.CacheAvailabilityPrefix == "" {
		return fmt.Errorf(
			"cache availability: invalid option: value %s was %s, where it should be %s",
			"CacheAvailabilityPrefix",
			"is empty",  // current value
			"non empty", // what the should be
		)
	}

	return nil
}

// WithTimeout is a functional option that the Availability interface
// implementers use to set the WriteBatchSize configuration param
//
// To be used with the construction
// example:
//
// NewShareAvailability(
//
//	bServ,
//	disc,
//	WithWriteBatchSize(uint),
//
// )
func WithWriteBatchSize(writeBatchSize uint) Option {
	return func(p *Parameters) {
		p.WriteBatchSize = writeBatchSize
	}
}

// WithCacheAvailabilityPrefix is a functional option that the Availability interface
// implementers use to set the CacheAvailabilityPrefix configuration param
//
// To be used with the construction, see example in WithWriteBatchSize documentation
func WithCacheAvailabilityPrefix(prefix string) Option {
	return func(p *Parameters) {
		p.CacheAvailabilityPrefix = prefix
	}
}
