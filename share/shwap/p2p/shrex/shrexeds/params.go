package shrexeds

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
)

const protocolString = shrex.ProtocolString + "eds" // will be removed soon

var log = logging.Logger("shrex/eds")

// Parameters is the set of parameters that must be configured for the shrex/eds protocol.
type Parameters struct {
	*shrex.Parameters

	// BufferSize defines the size of the buffer used for writing an ODS over the stream.
	BufferSize uint64
}

func DefaultParameters() *Parameters {
	return &Parameters{
		Parameters: shrex.DefaultParameters(),
		BufferSize: 32 * 1024,
	}
}

func (p *Parameters) Validate() error {
	if p.BufferSize <= 0 {
		return fmt.Errorf("invalid buffer size: %v, value should be positive and non-zero", p.BufferSize)
	}

	return p.Parameters.Validate()
}

func (c *Client) WithMetrics() error {
	metrics, err := shrex.InitClientMetrics()
	if err != nil {
		return fmt.Errorf("shrex/eds: init Metrics: %w", err)
	}
	c.metrics = metrics
	return nil
}

func (s *Server) WithMetrics() error {
	metrics, err := shrex.InitServerMetrics()
	if err != nil {
		return fmt.Errorf("shrex/eds: init Metrics: %w", err)
	}
	s.metrics = metrics
	return nil
}
