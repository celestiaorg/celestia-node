package header

import (
	"github.com/celestiaorg/celestia-core/pkg/da"
	core "github.com/celestiaorg/celestia-core/types"
)

// ExtendedHeader represents a wrapped "raw" header that includes
// information necessary for Celestia Nodes to be notified of new
// block headers and perform Data Availability Sampling.
type ExtendedHeader struct {
	*RawHeader
	DAH *da.DataAvailabilityHeader
}

// RawHeader is an alias to core.Header. It is
// "raw" because it is not yet wrapped to include
// the DataAvailabilityHeader.
type RawHeader = core.Header
