package shrexnd

import (
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const protocolString = "/shrex/nd/0.0.1"

var log = logging.Logger("shrex/nd")

// Option is the functional option that is applied to the shrex/eds protocol to configure its
// parameters.
type Option func(*Parameters)

// Parameters is the set of parameters that must be configured for the shrex/eds protocol.
type Parameters struct {
	// serverReadTimeout sets the timeout for reading messages from the stream on server side
	serverReadTimeout time.Duration

	// serverWriteTimeout sets the timeout for writing messages to the stream on server side
	serverWriteTimeout time.Duration

	// serveTimeout defines the deadline for serving request.
	serveTimeout time.Duration

	// networkID is prepended to the protocolID and represents the network the protocol is
	// running on.
	networkID string

	// concurrencyLimit is the maximum number of concurrently handled streams
	concurrencyLimit int
}

func DefaultParameters() *Parameters {
	return &Parameters{
		serverReadTimeout:  time.Second * 5,
		serverWriteTimeout: time.Minute, // based on max observed sample time for 256 blocks (~50s)
		serveTimeout:       time.Minute,
		concurrencyLimit:   10,
	}
}

const errSuffix = "value should be positive and non-zero"

func (p *Parameters) Validate() error {
	if p.serverReadTimeout <= 0 {
		return fmt.Errorf("invalid stream read timeout: %v, %s", p.serverReadTimeout, errSuffix)
	}
	if p.serverWriteTimeout <= 0 {
		return fmt.Errorf("invalid write timeout: %v, %s", p.serverWriteTimeout, errSuffix)
	}
	if p.serveTimeout <= 0 {
		return fmt.Errorf("invalid serve timeout: %v, %s", p.serveTimeout, errSuffix)
	}
	if p.concurrencyLimit <= 0 {
		return fmt.Errorf("invalid concurrency limit: %v, %s", p.concurrencyLimit, errSuffix)
	}
	return nil
}

// WithNetworkID is a functional option that configures the `networkID` parameter
func WithNetworkID(networkID string) Option {
	return func(parameters *Parameters) {
		parameters.networkID = networkID
	}
}

// WithReadTimeout is a functional option that configures the `readTimeout` parameter
func WithReadTimeout(readTimeout time.Duration) Option {
	return func(parameters *Parameters) {
		parameters.serverReadTimeout = readTimeout
	}
}

// WithWriteTimeout is a functional option that configures the `writeTimeout` parameter
func WithWriteTimeout(writeTimeout time.Duration) Option {
	return func(parameters *Parameters) {
		parameters.serverWriteTimeout = writeTimeout
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

func protocolID(networkID string) protocol.ID {
	return protocol.ID(fmt.Sprintf("/%s%s", networkID, protocolString))
}
