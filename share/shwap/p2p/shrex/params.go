package shrex

import (
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/protocol"
)

var log = logging.Logger("shrex")

// protocolString is the protocol string for the shrex protocol.
const protocolString = "/shrex/v0.1.0/"

// Parameters is the set of parameters that must be configured for the shrex protocol.
type Parameters struct {
	// ServerReadTimeout sets the timeout for reading messages from the stream.
	ServerReadTimeout time.Duration

	// ServerWriteTimeout sets the timeout for writing messages to the stream.
	ServerWriteTimeout time.Duration

	// HandleRequestTimeout defines the deadline for handling request.
	HandleRequestTimeout time.Duration

	// ConcurrencyLimit is the maximum number of concurrently handled streams
	ConcurrencyLimit int

	// networkID is prepended to the protocolID and represents the network the protocol is
	// running on.
	networkID string
}

func DefaultParameters() *Parameters {
	return &Parameters{
		ServerReadTimeout:    5 * time.Second,
		ServerWriteTimeout:   time.Minute, // based on max observed sample time for 256 blocks (~50s)
		HandleRequestTimeout: time.Minute,
		ConcurrencyLimit:     10,
	}
}

const errSuffix = "value should be positive and non-zero"

func (p *Parameters) Validate() error {
	if p.ServerReadTimeout <= 0 {
		return fmt.Errorf("invalid stream read timeout: %v, %s", p.ServerReadTimeout, errSuffix)
	}
	if p.ServerWriteTimeout <= 0 {
		return fmt.Errorf("invalid write timeout: %v, %s", p.ServerWriteTimeout, errSuffix)
	}
	if p.HandleRequestTimeout <= 0 {
		return fmt.Errorf("invalid handle request timeout: %v, %s", p.HandleRequestTimeout, errSuffix)
	}
	if p.ConcurrencyLimit <= 0 {
		return fmt.Errorf("invalid concurrency limit: %s", errSuffix)
	}
	return nil
}

// WithNetworkID sets the value of networkID in params
func (p *Parameters) WithNetworkID(networkID string) {
	p.networkID = networkID
}

// NetworkID returns the value of networkID stored in params
func (p *Parameters) NetworkID() string {
	return p.networkID
}

// ProtocolID creates a protocol ID string according to common format
func ProtocolID(networkID, protocolName string) protocol.ID {
	return protocol.ID(fmt.Sprintf("/%s%s%s", networkID, protocolString, protocolName))
}
