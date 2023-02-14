package shrexeds

import (
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const protocolPrefix = "/shrex/eds/v0.0.1/"

var log = logging.Logger("shrex-eds")

// Option is the functional option that is applied to the shrex/eds protocol to configure its
// parameters.
type Option func(*Parameters)

// Parameters is the set of parameters that must be configured for the shrex/eds protocol.
type Parameters struct {
	// ReadDeadline sets the timeout for reading messages from the stream.
	ReadDeadline time.Duration

	// WriteDeadline sets the timeout for writing messages to the stream.
	WriteDeadline time.Duration

	// ReadCARDeadline defines the deadline for reading a CAR from disk.
	ReadCARDeadline time.Duration

	// BufferSize defines the size of the buffer used for writing an ODS over the stream.
	BufferSize uint64

	// protocolSuffix is appended to the protocolID and represents the network the protocol is
	// running on.
	protocolSuffix string

	// concurrencyLimit is the maximum number of concurrently handled streams
	concurrencyLimit int
}

func DefaultParameters() *Parameters {
	return &Parameters{
		ReadDeadline:     time.Minute,
		WriteDeadline:    time.Second * 5,
		ReadCARDeadline:  time.Minute,
		BufferSize:       32 * 1024,
		concurrencyLimit: 10,
	}
}

const errSuffix = "value should be positive and non-zero"

func (p *Parameters) Validate() error {
	if p.ReadDeadline <= 0 {
		return fmt.Errorf("invalid stream read deadline: %s", errSuffix)
	}
	if p.WriteDeadline <= 0 {
		return fmt.Errorf("invalid write deadline: %s", errSuffix)
	}
	if p.ReadCARDeadline <= 0 {
		return fmt.Errorf("invalid read CAR deadline: %s", errSuffix)
	}
	if p.BufferSize <= 0 {
		return fmt.Errorf("invalid buffer size: %s", errSuffix)
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

// WithConcurrencyLimit is a functional option that configures the `concurrencyLimit` parameter
func WithConcurrencyLimit(concurrencyLimit int) Option {
	return func(parameters *Parameters) {
		parameters.concurrencyLimit = concurrencyLimit
	}
}

func protocolID(protocolSuffix string) protocol.ID {
	return protocol.ID(fmt.Sprintf("%s%s", protocolPrefix, protocolSuffix))
}
