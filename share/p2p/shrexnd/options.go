package shrexnd

import (
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const protocolPrefix = "/shrex/nd/0.0.1/"

var log = logging.Logger("shrex/nd")

// Option is the functional option that is applied to the shrex/eds protocol to configure its
// parameters.
type Option func(*Parameters)

// Parameters is the set of parameters that must be configured for the shrex/eds protocol.
type Parameters struct {
	// readTimeout sets the timeout for reading messages from the stream.
	readTimeout time.Duration

	// writeTimeout sets the timeout for writing messages to the stream.
	writeTimeout time.Duration

	// serveTimeout defines the deadline for serving request.
	serveTimeout time.Duration

	// protocolSuffix is appended to the protocolID and represents the network the protocol is
	// running on.
	protocolSuffix string

	// concurrencyLimit is the maximum number of concurrently handled streams
	concurrencyLimit int
}

func DefaultParameters() *Parameters {
	return &Parameters{
		readTimeout:      time.Second * 5,
		writeTimeout:     time.Second * 10,
		serveTimeout:     time.Second * 10,
		concurrencyLimit: 10,
	}
}

const errSuffix = "value should be positive and non-zero"

func (p *Parameters) Validate() error {
	if p.readTimeout <= 0 {
		return fmt.Errorf("invalid stream read timeout: %v, %s", p.readTimeout, errSuffix)
	}
	if p.writeTimeout <= 0 {
		return fmt.Errorf("invalid write timeout: %v, %s", p.writeTimeout, errSuffix)
	}
	if p.serveTimeout <= 0 {
		return fmt.Errorf("invalid serve timeout: %v, %s", p.serveTimeout, errSuffix)
	}
	if p.concurrencyLimit < 1 {
		return fmt.Errorf("invalid concurrency limit: value should be greater than 0")
	}
	return nil
}

// WithProtocolSuffix is a functional option that configures the `protocolSuffix` parameter
func WithProtocolSuffix(protocolSuffix string) Option {
	return func(parameters *Parameters) {
		parameters.protocolSuffix = protocolSuffix
	}
}

// WithReadTimeout is a functional option that configures the `readTimeout` parameter
func WithReadTimeout(readTimeout time.Duration) Option {
	return func(parameters *Parameters) {
		parameters.readTimeout = readTimeout
	}
}

// WithWriteTimeout is a functional option that configures the `writeTimeout` parameter
func WithWriteTimeout(writeTimeout time.Duration) Option {
	return func(parameters *Parameters) {
		parameters.writeTimeout = writeTimeout
	}
}

// WithServeTimeout is a functional option that configures the `serveTimeout` parameter
func WithServeTimeout(serveTimeout time.Duration) Option {
	return func(parameters *Parameters) {
		parameters.serveTimeout = serveTimeout
	}
}

// WithConcurrencyLimit is a functional option that configures the `concurrencyLimit` parameter
func WithConcurrencyLimit(concurrencyLimit int) Option {
	return func(parameters *Parameters) {
		parameters.concurrencyLimit = concurrencyLimit
	}
}

func protocolID(protocolSuffix string) protocol.ID {
	return protocol.ID(fmt.Sprintf("%s%s", protocolPrefix, protocolSuffix))
}
