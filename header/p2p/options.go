package p2p

import (
	"fmt"
	"time"
)

// Option is the functional option that is applied to the exchange instance
// to configure parameters.
type Option func(*Parameters)

// Parameters is the set of parameters that must be configured for the exchange.
type Parameters struct {
	// writeDeadline sets timeout for sending messages to the stream
	WriteDeadline time.Duration
	// readDeadline sets timeout for reading messages from the stream
	ReadDeadline time.Duration
	// the target minimum amount of responses with the same chain head
	MinResponses int
	// requestSize defines the max amount of headers that can be requested/handled at once.
	MaxRequestSize uint64
}

// DefaultParameters returns the default params to configure the store.
func DefaultParameters() *Parameters {
	return &Parameters{
		WriteDeadline:  time.Second * 5,
		ReadDeadline:   time.Minute,
		MinResponses:   2,
		MaxRequestSize: 512,
	}
}

const errSuffix = "value should be positive and non-zero"

func (p *Parameters) Validate() error {
	if p.WriteDeadline == 0 {
		return fmt.Errorf("invalid write time duration: %s", errSuffix)
	}
	if p.ReadDeadline == 0 {
		return fmt.Errorf("invalid read time duration: %s", errSuffix)
	}
	if p.MinResponses <= 0 {
		return fmt.Errorf("invalid minimal amount of responses: %s", errSuffix)
	}
	if p.MaxRequestSize == 0 {
		return fmt.Errorf("invalid max request size: %s", errSuffix)
	}
	return nil
}

// WithWriteDeadline is a functional option that configures the
// `WriteDeadline` parameter.
func WithWriteDeadline(deadline time.Duration) Option {
	return func(p *Parameters) {
		p.WriteDeadline = deadline
	}
}

// WithReadDeadline is a functional option that configures the
// `WithReadDeadline` parameter.
func WithReadDeadline(deadline time.Duration) Option {
	return func(p *Parameters) {
		p.ReadDeadline = deadline
	}
}

// WithMinResponses is a functional option that configures the
// `MinResponses` parameter.
func WithMinResponses(responses int) Option {
	return func(p *Parameters) {
		p.MinResponses = responses
	}
}

// WithMaxRequestSize is a functional option that configures the
// // `MaxRequestSize` parameter.
func WithMaxRequestSize(size uint64) Option {
	return func(p *Parameters) {
		p.MaxRequestSize = size
	}
}
