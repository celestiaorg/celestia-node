package availability

import (
	"fmt"
	"time"

	"github.com/celestiaorg/celestia-node/share"
)

var (
	// ErrInvalidAvailOption is an error that is returned by FullAvailabilityPameters.Validate
	// LightAvailabilityPameters.Validate and ShareParameters.Validate when supplied with invalid values.
	// This error will also be returned by NewShareAvailability if supplied with an invalid option
	ErrInvalidAvailOption = fmt.Errorf("availability: invalid option")

	// ErrInvalidAvailOption is an error that is returned by DiscoveryParameters.Validate
	// and ShareParameters.Validate when supplied with invalid values.
	// This error will also be returned by NewDiscovery if supplied with an invalid option
	ErrInvalidDiscOption = fmt.Errorf("discovery: invalid option")
)

// ErrInvalidOption is a utility function to dedup code for error-returning
// when dealing with invalid parameter values.
func ErrInvalidOption(err error, optionName string, value string) error {
	return fmt.Errorf("%w: value %s cannot be %s", err, optionName, value)
}

// Parameters is an interface tjat encompasses all params needed for the share package
type Parameters interface {
	FullAvailabilityParameters | LightAvailabilityParameters | CacheAvailabilityParamaters | DiscoveryParameters
}

// Option is a function that configures a ShareParameters
type Option[T Parameters] func(*T)

// FullAvailabilityParameters is the set of parameters that must be configured for the full availability implementation
type FullAvailabilityParameters struct {
	// AvailabilityTimeout specifies timeout for DA validation during which data have to be found on the network,
	// otherwise ErrNotAvailable is fired.
	// TODO: https://github.com/celestiaorg/celestia-node/issues/10)
	AvailabilityTimeout time.Duration
}

// LightAvailabilityParameters is the set of parameters that must be configured for the light availability implementation
type LightAvailabilityParameters struct {
	FullAvailabilityParameters
	SampleAmount uint // The minimum required amount of samples to perform
}

// CacheAvailabilityParameters is the set of parameters that must be configured for cache availability implementation
type CacheAvailabilityParamaters struct {
	// DefaultWriteBatchSize defines the size of the batched header write.
	// Headers are written in batches not to thrash the underlying Datastore with writes.
	// TODO(@Wondertan, @renaynay): Those values must be configurable and proper defaults should be set for specific node
	//  type. (#709)
	WriteBatchSize uint
	// The string prefix to use as a key for the datastore
	CacheAvailabilityPrefix string
}

// DiscoveryParameters is the set of parameters that must be configured for the discovery module
type DiscoveryParameters struct {
	// peersLimit is max amount of peers that will be discovered during a discovery session.
	PeersLimit uint

	// discInterval is an interval between discovery sessions.
	DiscoveryInterval time.Duration

	// advertiseInterval is an interval between advertising sessions.
	AdvertiseInterval time.Duration
}

// // ShareParameters is the aggergation of all configuration parameters of the the share module
type ShareParameters struct {
	// FullAvailabilityParameters
	LightAvailabilityParameters
	CacheAvailabilityParamaters
	DiscoveryParameters
}

// DefaultFullAvailabilityParameters returns default configuration parameter values
// for the full availability implementation
func DefaultFullAvailabilityParameters() FullAvailabilityParameters {
	return FullAvailabilityParameters{
		AvailabilityTimeout: share.DefaultAvailabilityTimeout,
	}
}

// Validate validates the values in FullAvailabilityParameters
func (p *FullAvailabilityParameters) Validate() error {
	if p.AvailabilityTimeout <= 0 {
		return ErrInvalidOption(
			ErrInvalidAvailOption,
			"Timeout",
			"native or 0",
		)
	}

	return nil
}

// WithAvailabilityTimeout is a functional option that the Availability interface
// implementers use to set the AvailabilityTimeout configuration param
//
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
func WithAvailabilityTimeout[T Parameters](timeout time.Duration) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) {
		case *FullAvailabilityParameters:
		case *LightAvailabilityParameters:
			t.AvailabilityTimeout = timeout
		}
	}
}

// DefaultLightAvailabilityParameters returns the default parameters' configuration values
// for the light availability implementation
func DefaultLightAvailabilityParameters() LightAvailabilityParameters {
	return LightAvailabilityParameters{
		FullAvailabilityParameters: DefaultFullAvailabilityParameters(),
		SampleAmount:               share.DefaultSampleAmount,
	}
}

// Validate validates the values in LightAvailabilityParameters
func (p *LightAvailabilityParameters) Validate() error {
	if err := p.FullAvailabilityParameters.Validate(); err != nil {
		return err
	}

	if p.SampleAmount <= 0 {
		return ErrInvalidOption(
			ErrInvalidAvailOption,
			"SampleAmount",
			"negative or 0",
		)
	}

	return nil
}

// WithSampleAmount is a functional option that the Availability interface
// implementers use to set the SampleAmount configuration param
//
// To be used with the construction, see example in WithAvailabilityTimeout documentation
func WithSampleAmount[T Parameters](sampleAmount uint) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) {
		case *LightAvailabilityParameters:
			t.SampleAmount = sampleAmount
		}
	}
}

// DefaultCacheAvailabilityParameters returns the default parameters' configuration values
// for the light availability implementation
func DefaultCacheAvailabilityParameters() CacheAvailabilityParamaters {
	return CacheAvailabilityParamaters{
		WriteBatchSize:          share.DefaultWriteBatchSize,
		CacheAvailabilityPrefix: share.DefaultCacheAvailabilityPrefix,
	}
}

// Validate validates the values in CacheAvailabilityParameters
func (ca *CacheAvailabilityParamaters) Validate() error {
	if ca.WriteBatchSize <= 0 {
		return ErrInvalidOption(
			ErrInvalidAvailOption,
			"DefaultWriteBatchSize",
			"negative or 0",
		)
	}

	if ca.CacheAvailabilityPrefix == "" {
		return ErrInvalidOption(
			ErrInvalidAvailOption,
			"CacheAvailabilityPrefix",
			"cannot be empty",
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
func WithWriteBatchSize[T Parameters](writeBatchSize uint) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) {
		case *CacheAvailabilityParamaters:
			t.WriteBatchSize = writeBatchSize
		}
	}
}

// WithCacheAvailabilityPrefix is a functional option that the Availability interface
// implementers use to set the CacheAvailabilityPrefix configuration param
//
// To be used with the construction, see example in WithWriteBatchSize documentation
func WithCacheAvailabilityPrefix[T Parameters](prefix string) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) {
		case *CacheAvailabilityParamaters:
			t.CacheAvailabilityPrefix = prefix
		}
	}
}

// DefaultDiscoveryParameters returns the default parameters' configuration values
// for the discovery module
func DefaultDiscoveryParameters() DiscoveryParameters {
	return DiscoveryParameters{
		PeersLimit:        3,
		DiscoveryInterval: time.Second * 30,
		AdvertiseInterval: time.Second * 30,
	}
}

// Validate validates the values in DiscoveryParameters
func (p *DiscoveryParameters) Validate() error {
	if p.PeersLimit <= 0 {
		return ErrInvalidOption(
			ErrInvalidDiscOption,
			"PeersLimit",
			"negative or 0",
		)
	}

	if p.DiscoveryInterval <= 0 {
		return ErrInvalidOption(
			ErrInvalidDiscOption,
			"DiscoveryInterval",
			"negative or 0",
		)
	}

	if p.AdvertiseInterval <= 0 {
		return ErrInvalidOption(
			ErrInvalidDiscOption,
			"AdvertiseInterval",
			"negative or 0",
		)
	}

	return nil
}

// WithPeersLimit is a functional option that the Discoverable interface
// implementers use to set the PeersLimit configuration param
//
// To be used with the construction
// example:
//
// NewDiscovery(
//
//	bServ,
//	disc,
//	WithPeersLimit(3),
//
// )
func WithPeersLimit[T Parameters](peersLimit uint) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) {
		case *DiscoveryParameters:
			t.PeersLimit = peersLimit
		}
	}
}

// WithDiscoveryInterval is a functional option that the Discoverable interface
// implementers use to set the DiscoveryInterval configuration param
//
// To be used with the construction, see example in WithPeersLimit documentation
func WithDiscoveryInterval[T Parameters](discInterval time.Duration) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) {
		case *DiscoveryParameters:
			t.DiscoveryInterval = discInterval
		}
	}
}

// WithAdvertiseInterval is a functional option that the Discoverable interface
// implementers use to set the AdvertiseInterval configuration param
//
// To be used with the construction, see example in WithPeersLimit documentation
func WithAdvertiseInterval[T Parameters](advInterval time.Duration) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) {
		case *DiscoveryParameters:
			t.AdvertiseInterval = advInterval
		}
	}
}

// DefaultDiscoveryParameters returns the default parameters' configuration values
// for all modules in the share module
func DefaultShareParameters() ShareParameters {
	return ShareParameters{
		LightAvailabilityParameters: DefaultLightAvailabilityParameters(),
		CacheAvailabilityParamaters: DefaultCacheAvailabilityParameters(),
		DiscoveryParameters:         DefaultDiscoveryParameters(),
	}
}

// Validate validates the values in ShareParameters
func (p *ShareParameters) Validate() error {
	err := p.LightAvailabilityParameters.Validate()
	if err != nil {
		return err
	}

	err = p.CacheAvailabilityParamaters.Validate()
	if err != nil {
		return err
	}

	err = p.DiscoveryParameters.Validate()
	if err != nil {
		return err
	}

	return nil
}
