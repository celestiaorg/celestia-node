package header

import (
	"fmt"

	tmbytes "github.com/celestiaorg/celestia-core/libs/bytes"
	"github.com/celestiaorg/celestia-core/pkg/da"
	core "github.com/celestiaorg/celestia-core/types"
)

// RawHeader is an alias to core.Header. It is
// "raw" because it is not yet wrapped to include
// the DataAvailabilityHeader.
type RawHeader = core.Header

// ExtendedHeader represents a wrapped "raw" header that includes
// information necessary for Celestia Nodes to be notified of new
// block headers and perform Data Availability Sampling.
type ExtendedHeader struct {
	RawHeader    `json:"header"`
	Commit       *core.Commit               `json:"commit"`
	ValidatorSet *core.ValidatorSet         `json:"validator_set"`
	DAH          *da.DataAvailabilityHeader `json:"dah"`
}

// MarshalBinary marshals ExtendedHeader to binary.
func (eh *ExtendedHeader) MarshalBinary() ([]byte, error) {
	return MarshalExtendedHeader(eh)
}

// MarshalBinary unmarshals ExtendedHeader from binary.
func (eh *ExtendedHeader) UnmarshalBinary(data []byte) error {
	if eh == nil {
		return fmt.Errorf("header: cannot UnmarshalBinary - nil ExtendedHeader")
	}

	out, err := UnmarshalExtendedHeader(data)
	if err != nil {
		return err
	}

	*eh = *out
	return nil
}

// ExtendedHeaderRequest represents a request for one or more
// ExtendedHeaders from the Celestia network.
type ExtendedHeaderRequest struct {
	Origin int64 // block height from which to begin retrieving headers (in descending order)
	Amount int   // quantity of headers to be requested
}

// Hash is an alias to HexBytes which enables HEX-encoding for
// json/encoding.
type Hash = tmbytes.HexBytes
