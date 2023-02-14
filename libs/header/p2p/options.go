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
	// MaxRequestSize defines the max amount of headers that can be handled at once.
	MaxRequestSize uint64
	// RequestTimeout defines a timeout after which the session will try to re-request headers
	// from another peer.
	RequestTimeout time.Duration
	// protocolSuffix is a network suffix that will be used to create a protocol.ID
	// Is empty by default
	protocolSuffix string
	// EnableMetrics enables metrics collection for the exchange.
	EnableMetrics bool
}

// DefaultServerParameters returns the default params to configure the store.
func DefaultServerParameters() ServerParameters {
	return ServerParameters{
		WriteDeadline:  time.Second * 5,
		ReadDeadline:   time.Minute,
		MaxRequestSize: 512,
		RequestTimeout: time.Second * 5,
	}
}

func (p *ServerParameters) Validate() error {
	if p.WriteDeadline == 0 {
		return fmt.Errorf("invalid write time duration: %v", p.WriteDeadline)
	}
	if p.ReadDeadline == 0 {
		return fmt.Errorf("invalid read time duration: %v", p.ReadDeadline)
	}
	if p.MaxRequestSize == 0 {
		return fmt.Errorf("invalid max request size: %d", p.MaxRequestSize)
	}
	if p.RequestTimeout == 0 {
		return fmt.Errorf("invalid request timeout for session: "+
			"%s. %s: %v", greaterThenZero, providedSuffix, p.RequestTimeout)
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

// WithMaxRequestSize is a functional option that configures the
// `MaxRequestSize` parameter.
func WithMaxRequestSize[T parameters](size uint64) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) {
		case *ClientParameters:
			t.MaxRequestSize = size
		case *ServerParameters:
			t.MaxRequestSize = size
		}
	}
}

// WithRequestTimeout is a functional option that configures the
// `RequestTimeout` parameter.
func WithRequestTimeout[T parameters](duration time.Duration) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) {
		case *ClientParameters:
			t.RequestTimeout = duration
		case *ServerParameters:
			t.RequestTimeout = duration
		}
	}
}

// WithProtocolSuffix is a functional option that configures the
// `protocolSuffix` parameter.
func WithProtocolSuffix[T parameters](protocolSuffix string) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) {
		case *ClientParameters:
			t.protocolSuffix = protocolSuffix
		case *ServerParameters:
			t.protocolSuffix = protocolSuffix
		}
	}
}

// ClientParameters is the set of parameters that must be configured for the exchange.
// TODO: #1667
type ClientParameters struct {
	// the target minimum amount of responses with the same chain head
	MinResponses int
	// MaxRequestSize defines the max amount of headers that can be handled at once.
	MaxRequestSize uint64 // TODO: Rename to MaxRangeRequestSize
	// MaxHeadersPerRequest defines the max amount of headers that can be requested per 1 request.
	MaxHeadersPerRequest uint64 // TODO: Rename to MaxHeadersPerRangeRequest
	// MaxAwaitingTime specifies the duration that gives to the disconnected peer to be back online,
	// otherwise it will be removed on the next GC cycle.
	MaxAwaitingTime time.Duration
	// DefaultScore specifies the score for newly connected peers.
	DefaultScore float32
	// RequestTimeout defines a timeout after which the session will try to re-request headers
	// from another peer.
	RequestTimeout time.Duration // TODO: Rename to RangeRequestTimeout
	// TrustedPeersRequestTimeout a timeout for any request to a trusted peer.
	TrustedPeersRequestTimeout time.Duration
	// MaxTrackerSize specifies the max amount of peers that can be added to the peerTracker.
	MaxPeerTrackerSize int
	// protocolSuffix is a network suffix that will be used to create a protocol.ID
	protocolSuffix string
	// EnableMetrics enables metrics collection for the exchange.
	EnableMetrics bool
}

// DefaultClientParameters returns the default params to configure the store.
func DefaultClientParameters() ClientParameters {
	return ClientParameters{
		MinResponses:               2,
		MaxRequestSize:             512,
		MaxHeadersPerRequest:       64,
		MaxAwaitingTime:            time.Hour,
		DefaultScore:               1,
		RequestTimeout:             time.Second * 3,
		TrustedPeersRequestTimeout: time.Millisecond * 300,
		MaxPeerTrackerSize:         100,
	}
}

const (
	greaterThenZero = "should be greater than 0"
	providedSuffix  = "Provided value"
)

func (p *ClientParameters) Validate() error {
	if p.MinResponses <= 0 {
		return fmt.Errorf("invalid MinResponses: %s. %s: %v",
			greaterThenZero, providedSuffix, p.MinResponses)
	}
	if p.MaxRequestSize == 0 {
		return fmt.Errorf("invalid MaxRequestSize: %s. %s: %v", greaterThenZero, providedSuffix, p.MaxRequestSize)
	}
	if p.MaxHeadersPerRequest == 0 {
		return fmt.Errorf("invalid MaxHeadersPerRequest: %s. %s: %v", greaterThenZero, providedSuffix, p.MaxHeadersPerRequest)
	}
	if p.MaxHeadersPerRequest > p.MaxRequestSize {
		return fmt.Errorf("MaxHeadersPerRequest should not be more than MaxRequestSize."+
			"MaxHeadersPerRequest: %v, MaxRequestSize: %v", p.MaxHeadersPerRequest, p.MaxRequestSize)
	}
	if p.MaxAwaitingTime == 0 {
		return fmt.Errorf("invalid MaxAwaitingTime for peerTracker: "+
			"%s. %s: %v", greaterThenZero, providedSuffix, p.MaxAwaitingTime)
	}
	if p.DefaultScore <= 0 {
		return fmt.Errorf("invalid DefaultScore: %s. %s: %f", greaterThenZero, providedSuffix, p.DefaultScore)
	}
	if p.RequestTimeout == 0 {
		return fmt.Errorf("invalid request timeout for session: "+
			"%s. %s: %v", greaterThenZero, providedSuffix, p.RequestTimeout)
	}
	if p.TrustedPeersRequestTimeout == 0 {
		return fmt.Errorf("invalid TrustedPeersRequestTimeout: "+
			"%s. %s: %v", greaterThenZero, providedSuffix, p.TrustedPeersRequestTimeout)
	}
	if p.MaxPeerTrackerSize <= 0 {
		return fmt.Errorf("invalid MaxTrackerSize: %s. %s: %d", greaterThenZero, providedSuffix, p.MaxPeerTrackerSize)
	}
	return nil
}

// WithMinResponses is a functional option that configures the
// `MinResponses` parameter.
func WithMinResponses[T ClientParameters](responses int) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) { //nolint:gocritic
		case *ClientParameters:
			t.MinResponses = responses
		}
	}
}

// WithMaxHeadersPerRequest is a functional option that configures the
// // `MaxRequestSize` parameter.
func WithMaxHeadersPerRequest[T ClientParameters](amount uint64) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) { //nolint:gocritic
		case *ClientParameters:
			t.MaxHeadersPerRequest = amount
		}

	}
}

// WithMaxAwaitingTime is a functional option that configures the
// `MaxAwaitingTime` parameter.
func WithMaxAwaitingTime[T ClientParameters](duration time.Duration) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) { //nolint:gocritic
		case *ClientParameters:
			t.MaxAwaitingTime = duration
		}
	}
}

// WithDefaultScore is a functional option that configures the
// `DefaultScore` parameter.
func WithDefaultScore[T ClientParameters](score float32) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) { //nolint:gocritic
		case *ClientParameters:
			t.DefaultScore = score
		}
	}
}

// WithMaxTrackerSize is a functional option that configures the
// `MaxTrackerSize` parameter.
func WithMaxTrackerSize[T ClientParameters](size int) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) { //nolint:gocritic
		case *ClientParameters:
			t.MaxPeerTrackerSize = size
		}
	}
}
