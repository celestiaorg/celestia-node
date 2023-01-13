package shrexnd

import (
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const protocolPrefix = "/shrex/nd/0.0.1"

var log = logging.Logger("shrex/nd")

// Option is the functional option that is applied to the shrex/eds protocol to configure its
// parameters.
type Option func(*Parameters)

// Parameters is the set of parameters that must be configured for the shrex/eds protocol.
type Parameters struct {
	// ReadTimeout sets the timeout for reading messages from the stream.
	ReadTimeout time.Duration

	// WriteTimeout sets the timeout for writing messages to the stream.
	WriteTimeout time.Duration

	// ServeTimeout defines the deadline for serving request.
	ServeTimeout time.Duration

	// protocolSuffix is appended to the protocolID and represents the network the protocol is
	// running on.
	protocolSuffix string
}

func DefaultParameters() *Parameters {
	return &Parameters{
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 10,
		ServeTimeout: time.Second * 10,
	}
}

const errSuffix = "value should be positive and non-zero"

func (p *Parameters) Validate() error {
	if p.ReadTimeout <= 0 {
		return fmt.Errorf("invalid stream read timeout: %v, %s", p.ReadTimeout, errSuffix)
	}
	if p.WriteTimeout <= 0 {
		return fmt.Errorf("invalid write timeout: %v, %s", p.WriteTimeout, errSuffix)
	}
	if p.ServeTimeout <= 0 {
		return fmt.Errorf("invalid serve timeout: %v, %s", p.ServeTimeout, errSuffix)
	}
	return nil
}

// WithProtocolSuffix is a functional option that configures the `protocolSuffix` parameter
func WithProtocolSuffix(protocolSuffix string) Option {
	return func(parameters *Parameters) {
		parameters.protocolSuffix = protocolSuffix
	}
}

func protocolID(protocolSuffix string) protocol.ID {
	return protocol.ID(fmt.Sprintf("%s%s", protocolPrefix, protocolSuffix))
}
