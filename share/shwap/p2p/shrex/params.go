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

type Parameters struct {
	// ReadTimeout sets the timeout for reading messages from the stream.
	ReadTimeout time.Duration

	// WriteTimeout sets the timeout for writing messages to the stream.
	WriteTimeout time.Duration

	// networkID is prepended to the protocolID and represents the network the protocol is
	// running on.
	networkID string
}

type ClientParams struct {
	*Parameters
}

type ServerParams struct {
	*Parameters

	// HandleRequestTimeout defines the deadline for handling request.
	HandleRequestTimeout time.Duration

	// ConcurrencyLimit is the maximum number of concurrently handled streams
	ConcurrencyLimit int
}

func DefaultClientParameters() *ClientParams {
	return &ClientParams{Parameters: defaultParameters()}
}
func DefaultServerParameters() *ServerParams {
	return &ServerParams{
		Parameters:           defaultParameters(),
		HandleRequestTimeout: time.Minute,
		ConcurrencyLimit:     10,
	}
}

func defaultParameters() *Parameters {
	return &Parameters{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: time.Minute, // based on max observed sample time for 256 blocks (~50s)

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
	return nil
}

func (c *ClientParams) Validate() error {
	return c.Parameters.Validate()
}

func (s *ServerParams) Validate() error {
	if err := s.Parameters.Validate(); err != nil {
		return err
	}
	if s.HandleRequestTimeout <= 0 {
		return fmt.Errorf("invalid handle request timeout: %v, %s", s.HandleRequestTimeout, errSuffix)
	}
	if s.ConcurrencyLimit <= 0 {
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
