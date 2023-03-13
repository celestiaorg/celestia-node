package p2p

import (
	"fmt"
	"time"
)

// parameters is an interface that encompasses all params needed for
// client and server parameters to protect `optional functions` from this package.
type parameters interface {
	ServerParameters | ClientParameters
}

// Option is the functional option that is applied to the exchange instance
// to configure parameters.
type Option[T parameters] func(*T)

// ServerParameters is the set of parameters that must be configured for the exchange.
type ServerParameters struct {
	// WriteDeadline sets the timeout for sending messages to the stream
	WriteDeadline time.Duration
	// ReadDeadline sets the timeout for reading messages from the stream
	ReadDeadline time.Duration
	// RangeRequestTimeout defines a timeout after which the session will try to re-request headers
	// from another peer.
	RangeRequestTimeout time.Duration
	// networkID is a network that will be used to create a protocol.ID
	// Is empty by default
	networkID string
}

// DefaultServerParameters returns the default params to configure the store.
func DefaultServerParameters() ServerParameters {
	return ServerParameters{
		WriteDeadline:       time.Second * 5,
		ReadDeadline:        time.Minute,
		RangeRequestTimeout: time.Second * 5,
	}
}

func (p *ServerParameters) Validate() error {
	if p.WriteDeadline == 0 {
		return fmt.Errorf("invalid write time duration: %v", p.WriteDeadline)
	}
	if p.ReadDeadline == 0 {
		return fmt.Errorf("invalid read time duration: %v", p.ReadDeadline)
	}
	if p.RangeRequestTimeout == 0 {
		return fmt.Errorf("invalid request timeout for session: "+
			"%s. %s: %v", greaterThenZero, providedSuffix, p.RangeRequestTimeout)
	}
	return nil
}

// WithWriteDeadline is a functional option that configures the
// `WriteDeadline` parameter.
func WithWriteDeadline[T ServerParameters](deadline time.Duration) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) { //nolint:gocritic
		case *ServerParameters:
			t.WriteDeadline = deadline
		}
	}
}

// WithReadDeadline is a functional option that configures the
// `WithReadDeadline` parameter.
func WithReadDeadline[T ServerParameters](deadline time.Duration) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) { //nolint:gocritic
		case *ServerParameters:
			t.ReadDeadline = deadline
		}
	}
}

// WithRangeRequestTimeout is a functional option that configures the
// `RangeRequestTimeout` parameter.
func WithRangeRequestTimeout[T parameters](duration time.Duration) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) {
		case *ClientParameters:
			t.RangeRequestTimeout = duration
		case *ServerParameters:
			t.RangeRequestTimeout = duration
		}
	}
}

// WithNetworkID is a functional option that configures the
// `networkID` parameter.
func WithNetworkID[T parameters](networkID string) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) {
		case *ClientParameters:
			t.networkID = networkID
		case *ServerParameters:
			t.networkID = networkID
		}
	}
}

// ClientParameters is the set of parameters that must be configured for the exchange.
type ClientParameters struct {
	// MaxHeadersPerRangeRequest defines the max amount of headers that can be requested per 1 request.
	MaxHeadersPerRangeRequest uint64
	// RangeRequestTimeout defines a timeout after which the session will try to re-request headers
	// from another peer.
	RangeRequestTimeout time.Duration
	// TrustedPeersRequestTimeout a timeout for any request to a trusted peer.
	TrustedPeersRequestTimeout time.Duration
	// networkID is a network that will be used to create a protocol.ID
	networkID string
	// chainID is an identifier of the chain.
	chainID string
}

// DefaultClientParameters returns the default params to configure the store.
func DefaultClientParameters() ClientParameters {
	return ClientParameters{
		MaxHeadersPerRangeRequest:  64,
		RangeRequestTimeout:        time.Second * 8,
		TrustedPeersRequestTimeout: time.Millisecond * 300,
	}
}

const (
	greaterThenZero = "should be greater than 0"
	providedSuffix  = "Provided value"
)

func (p *ClientParameters) Validate() error {
	if p.MaxHeadersPerRangeRequest == 0 {
		return fmt.Errorf("invalid MaxHeadersPerRangeRequest:%s. %s: %v",
			greaterThenZero, providedSuffix, p.MaxHeadersPerRangeRequest)
	}
	if p.RangeRequestTimeout == 0 {
		return fmt.Errorf("invalid request timeout for session: "+
			"%s. %s: %v", greaterThenZero, providedSuffix, p.RangeRequestTimeout)
	}
	if p.TrustedPeersRequestTimeout == 0 {
		return fmt.Errorf("invalid TrustedPeersRequestTimeout: "+
			"%s. %s: %v", greaterThenZero, providedSuffix, p.TrustedPeersRequestTimeout)
	}
	return nil
}

// WithMaxHeadersPerRangeRequest is a functional option that configures the
// // `MaxRangeRequestSize` parameter.
func WithMaxHeadersPerRangeRequest[T ClientParameters](amount uint64) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) { //nolint:gocritic
		case *ClientParameters:
			t.MaxHeadersPerRangeRequest = amount
		}

	}
}

// WithChainID is a functional option that configures the
// `chainID` parameter.
func WithChainID[T ClientParameters](chainID string) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) { //nolint:gocritic
		case *ClientParameters:
			t.chainID = chainID
		}
	}
}
