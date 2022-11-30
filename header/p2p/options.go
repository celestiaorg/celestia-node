package p2p

import (
	"errors"
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
	// GC defines the duration after which the peerTracker starts removing peers.
	GC time.Duration
	// MaxAwaitingTime specifies the duration that gives to the disconnected peer to be back online,
	// otherwise it will be removed on the next GC cycle.
	MaxAwaitingTime time.Duration
	// DefaultScore specifies the score for newly connected peers.
	DefaultScore float32
}

// DefaultClientParameters returns the default params to configure the store.
func DefaultClientParameters() *ClientParameters {
	return &ClientParameters{
		MinResponses:         2,
		MaxRequestSize:       512,
		MaxHeadersPerRequest: 64,
		GC:                   time.Minute * 30,
		MaxAwaitingTime:      time.Hour,
		DefaultScore:         1,
	}
}

func (p *ClientParameters) Validate() error {
	if p.MinResponses <= 0 {
		return errors.New("invalid minimum amount of responses: value should be positive and non-zero")
	}
	if p.MaxRequestSize == 0 {
		return fmt.Errorf("invalid max request size: %d", p.MaxRequestSize)
	}
	if p.MaxHeadersPerRequest == 0 || p.MaxHeadersPerRequest > p.MaxRequestSize {
		return errors.New("invalid max headers per request")
	}
	if p.GC == 0 {
		return fmt.Errorf("invalid gc period for peerTracker: %v", p.GC)
	}
	if p.MaxAwaitingTime == 0 {
		return fmt.Errorf("invalid gc maxAwaitingTime for peerTracker: %s", p.MaxAwaitingTime)
	}
	if p.DefaultScore < 0 {
		return fmt.Errorf("invalid default score %f", p.DefaultScore)
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

// WithGCCycle is a functional option that configures the
// `GC` parameter.
func WithGCCycle[T ClientParameters](cycle time.Duration) Option[T] {
	return func(p *T) {
		switch t := any(p).(type) { //nolint:gocritic
		case *ClientParameters:
			t.GC = cycle
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
