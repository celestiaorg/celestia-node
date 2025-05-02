package shrexnd

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
)

// Parameters is the set of parameters that must be configured for the shrex protocol.
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
