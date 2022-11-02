package availability

import (
	"fmt"
	"time"

	"github.com/celestiaorg/celestia-node/share"
)

var (
	// ErrInvalidAvailOption is an error that is returned by FullAvailPameters.Validate
	// LightAvailPameters.Validate and ShareParameters.Validate when supplied with invalid values.
	// This error will also be returned by NewShareAvailability if supplied with an invalid option
	ErrInvalidAvailOption = fmt.Errorf("availability: invalid option")

	// ErrInvalidAvailOption is an error that is returned by DiscoveryParameters.Validate
	// and ShareParameters.Validate when supplied with invalid values.
	// This error will also be returned by NewDiscovery if supplied with an invalid option
	ErrInvalidDiscOption = fmt.Errorf("discovery: invalid option")
)

// errInvalidAvailOptionValue is a utility function to dedup code for error-returning
// when dealing with invalid parameter values.
func errInvalidAvailOptionValue(optionName string, value string) error {
	return fmt.Errorf("%w: value %s cannot be %s", ErrInvalidAvailOption, optionName, value)
}

// errInvalidDiscOptionValue is a utility function to dedup code for error-returning
// when dealing with invalid parameter values.
func errInvalidDiscOptionValue(optionName string, value string) error {
	return fmt.Errorf("%w: value %s cannot be %s", ErrInvalidDiscOption, optionName, value)
}

// Option is the type for the functional options that ShareAvailability uses to set parameters
type AvailOption func(share.Availability)

// DiscOption is the type for the functional options that Discoverable uses to set parameters
type DiscOption func(share.Discoverable)

// FullAvailParameters is the set of parameters that must be configured for the full availability implementation
type FullAvailParameters struct {
	// AvailabilityTimeout specifies timeout for DA validation during which data have to be found on the network,
	// otherwise ErrNotAvailable is fired.
	// TODO: https://github.com/celestiaorg/celestia-node/issues/10)
	AvailabilityTimeout time.Duration
}

// LightAvailParameters is the set of parameters that must be configured for the light availability implementation
type LightAvailParameters struct {
	FullAvailParameters
	SampleAmount uint // The minimum required amount of samples to perform
}

// CacheAvailParameters is the set of parameters that must be configured for cache availability implementation
type CacheAvailParamaters struct {
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

// ShareParameters is the aggergation of all configuration parameters of the the share module
type ShareParameters struct {
	LightAvailParameters
	CacheAvailParamaters
	DiscoveryParameters
}

// DefaultFullAvailParameters returns default configuration parameter values
// for the full availability implementation
func DefaultFullAvailParameters() FullAvailParameters {
	return FullAvailParameters{
		AvailabilityTimeout: share.DefaultAvailabilityTimeout,
	}
}

// Validate validates the values in FullAvailParameters
func (p *FullAvailParameters) Validate() error {
	if p.AvailabilityTimeout <= 0 {
		return errInvalidAvailOptionValue(
			"Timeout",
			"native or 0",
		)
	}

	return nil
}

// DefaultLightAvailParameters returns the default parameters' configuration values
// for the light availability implementation
func DefaultLightAvailParameters() LightAvailParameters {
	return LightAvailParameters{
		FullAvailParameters: FullAvailParameters{
			AvailabilityTimeout: share.DefaultAvailabilityTimeout,
		},
		SampleAmount: share.DefaultSampleAmount,
	}
}

// Validate validates the values in LightAvailParameters
func (p *LightAvailParameters) Validate() error {
	err := p.FullAvailParameters.Validate()
	if err != nil {
		return nil
	}

	if p.SampleAmount <= 0 {
		return errInvalidAvailOptionValue(
			"SampleAmount",
			"negative or 0",
		)
	}

	return nil
}

// DefaultCacheAvailParameters returns the default parameters' configuration values
// for the light availability implementation
func DefaultCacheAvailParameters() CacheAvailParamaters {
	return CacheAvailParamaters{
		WriteBatchSize:          share.DefaultWriteBatchSize,
		CacheAvailabilityPrefix: share.DefaultCacheAvailabilityPrefix,
	}
}

// Validate validates the values in CacheAvailParameters
func (ca *CacheAvailParamaters) Validate() error {
	if ca.WriteBatchSize <= 0 {
		return errInvalidAvailOptionValue(
			"DefaultWriteBatchSize",
			"negative or 0",
		)
	}

	if ca.CacheAvailabilityPrefix == "" {
		return errInvalidAvailOptionValue(
			"CacheAvailabilityPrefix",
			"cannot be empty",
		)
	}

	return nil
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
		return errInvalidDiscOptionValue(
			"PeersLimit",
			"negative or 0",
		)
	}

	if p.DiscoveryInterval <= 0 {
		return errInvalidDiscOptionValue(
			"DiscoveryInterval",
			"negative or 0",
		)
	}

	if p.AdvertiseInterval <= 0 {
		return errInvalidDiscOptionValue(
			"AdvertiseInterval",
			"negative or 0",
		)
	}

	return nil
}

// DefaultDiscoveryParameters returns the default parameters' configuration values
// for all modules in the share module
func DefaultShareParameters() ShareParameters {
	return ShareParameters{
		LightAvailParameters: DefaultLightAvailParameters(),
		CacheAvailParamaters: DefaultCacheAvailParameters(),
		DiscoveryParameters:  DefaultDiscoveryParameters(),
	}
}

// Validate validates the values in ShareParameters
func (p *ShareParameters) Validate() error {
	err := p.LightAvailParameters.Validate()
	if err != nil {
		return err
	}

	err = p.CacheAvailParamaters.Validate()
	if err != nil {
		return err
	}

	err = p.DiscoveryParameters.Validate()
	if err != nil {
		return err
	}

	return nil
}

// WithTimeout is a functional option that the Availability interface
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
func WithAvailabilityTimeout(timeout time.Duration) AvailOption {
	return func(av share.Availability) {
		av.SetParam("AvailabilityTimeout", timeout)
	}
}

// WithSampleAmount is a functional option that the Availability interface
// implementers use to set the SampleAmount configuration param
//
// To be used with the construction, see example in WithAvailabilityTimeout documentation
func WithSampleAmount(sampleAmount uint) AvailOption {
	return func(av share.Availability) {
		av.SetParam("SampleAmount", sampleAmount)
	}
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
func WithWriteBatchSize(writeBatchSize uint) AvailOption {
	return func(av share.Availability) {
		av.SetParam("WriteBatchSize", writeBatchSize)
	}
}

// WithCacheAvailabilityPrefix is a functional option that the Availability interface
// implementers use to set the CacheAvailabilityPrefix configuration param
//
// To be used with the construction, see example in WithWriteBatchSize documentation
func WithCacheAvailabilityPrefix(prefix string) AvailOption {
	return func(av share.Availability) {
		av.SetParam("CacheAvailabilityPrefix", prefix)
	}
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
func WithPeersLimit(peersLimit uint) DiscOption {
	return func(disc share.Discoverable) {
		disc.SetParam("PeersLimit", peersLimit)
	}
}

// WithDiscoveryInterval is a functional option that the Discoverable interface
// implementers use to set the DiscoveryInterval configuration param
//
// To be used with the construction, see example in WithPeersLimit documentation
func WithDiscoveryInterval(discInterval time.Duration) DiscOption {
	return func(disc share.Discoverable) {
		disc.SetParam("DiscoveryInterval", discInterval)
	}
}

// WithAdvertiseInterval is a functional option that the Discoverable interface
// implementers use to set the AdvertiseInterval configuration param
//
// To be used with the construction, see example in WithPeersLimit documentation
func WithAdvertiseInterval(discInterval time.Duration) DiscOption {
	return func(disc share.Discoverable) {
		disc.SetParam("AdvertiseInterval", discInterval)
	}
}
