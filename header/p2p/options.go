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
}

// DefaultServerParameters returns the default params to configure the store.
func DefaultServerParameters() *ServerParameters {
	return &ServerParameters{
		WriteDeadline:  time.Second * 5,
		ReadDeadline:   time.Minute,
		MaxRequestSize: 512,
	}
}

const errSuffix = "value should be positive and non-zero"

func (p *ServerParameters) Validate() error {
	if p.WriteDeadline == 0 {
		return fmt.Errorf("invalid write time duration: %s", errSuffix)
	}
	if p.ReadDeadline == 0 {
		return fmt.Errorf("invalid read time duration: %s", errSuffix)
	}
	if p.MaxRequestSize == 0 {
		return fmt.Errorf("invalid max request size: %s", errSuffix)
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

// ClientParameters is the set of parameters that must be configured for the exchange.
type ClientParameters struct {
	// the target minimum amount of responses with the same chain head
	MinResponses int
	// MaxRequestSize defines the max amount of headers that can be handled at once.
	MaxRequestSize uint64
	// MaxHeadersPerRequest defines the max amount of headers that can be requested per 1 request.
	MaxHeadersPerRequest uint64
}

// DefaultClientParameters returns the default params to configure the store.
func DefaultClientParameters() *ClientParameters {
	return &ClientParameters{
		MinResponses:         2,
		MaxRequestSize:       512,
		MaxHeadersPerRequest: 64,
	}
}

func (p *ClientParameters) Validate() error {
	if p.MinResponses <= 0 {
		return fmt.Errorf("invalid minimum amount of responses: %s", errSuffix)
	}
	if p.MaxRequestSize == 0 {
		return fmt.Errorf("invalid max request size: %s", errSuffix)
	}
	if p.MaxHeadersPerRequest == 0 || p.MaxHeadersPerRequest > p.MaxRequestSize {
		return fmt.Errorf("invalid max headers per request: %s", errSuffix)
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
