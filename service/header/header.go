package header

import (
	"bytes"
	"fmt"

	bts "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/pkg/da"
	core "github.com/tendermint/tendermint/types"

	header_pb "github.com/celestiaorg/celestia-node/service/header/pb"
)

type DataAvailabilityHeader = da.DataAvailabilityHeader

// EmptyDAH provides DAH of the empty block.
var EmptyDAH = da.MinDataAvailabilityHeader

// RawHeader is an alias to core.Header. It is
// "raw" because it is not yet wrapped to include
// the DataAvailabilityHeader.
type RawHeader = core.Header

// ExtendedHeader represents a wrapped "raw" header that includes
// information necessary for Celestia Nodes to be notified of new
// block headers and perform Data Availability Sampling.
type ExtendedHeader struct {
	RawHeader    `json:"header"`
	Commit       *core.Commit            `json:"commit"`
	ValidatorSet *core.ValidatorSet      `json:"validator_set"`
	DAH          *DataAvailabilityHeader `json:"dah"`
}

// Hash returns Hash of the wrapped RawHeader.
// NOTE: It purposely overrides Hash method of RawHeader to get it directly from Commit without recomputing.
func (eh *ExtendedHeader) Hash() bts.HexBytes {
	return eh.Commit.BlockID.Hash
}

// LastHeader returns the Hash of the last wrapped RawHeader.
func (eh *ExtendedHeader) LastHeader() bts.HexBytes {
	return eh.RawHeader.LastBlockID.Hash
}

// ValidateBasic performs *basic* validation to check for missed/incorrect fields.
func (eh *ExtendedHeader) ValidateBasic() error {
	err := eh.RawHeader.ValidateBasic()
	if err != nil {
		return err
	}

	err = eh.Commit.ValidateBasic()
	if err != nil {
		return err
	}

	err = eh.ValidatorSet.ValidateBasic()
	if err != nil {
		return err
	}

	if eh.Commit.Height != eh.Height {
		return fmt.Errorf("header and commit height mismatch: %d vs %d", eh.Height, eh.Commit.Height)
	}
	if hhash, chash := eh.Hash(), eh.Commit.BlockID.Hash; !bytes.Equal(hhash, chash) {
		return fmt.Errorf("commit signs block %X, header is block %X", chash, hhash)
	}

	// make sure the validator set is consistent with the header
	if valSetHash := eh.ValidatorSet.Hash(); !bytes.Equal(eh.ValidatorsHash, valSetHash) {
		return fmt.Errorf("expected validator hash of header to match validator set hash (%X != %X)",
			eh.ValidatorsHash, valSetHash,
		)
	}

	return eh.DAH.ValidateBasic()
}

// MarshalBinary marshals ExtendedHeader to binary.
func (eh *ExtendedHeader) MarshalBinary() ([]byte, error) {
	return MarshalExtendedHeader(eh)
}

// UnmarshalBinary unmarshals ExtendedHeader from binary.
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

// ExtendedHeaderRequest is the packet format for nodes to request ExtendedHeaders
// from the network.
type ExtendedHeaderRequest struct {
	Origin uint64 // block height from which to request ExtendedHeaders
	Amount uint64 // amount of desired ExtendedHeaders starting from Origin, syncing in ascending order
}

// MarshalBinary marshals ExtendedHeaderRequest to binary.
func (ehr *ExtendedHeaderRequest) MarshalBinary() ([]byte, error) {
	return MarshalExtendedHeaderRequest(ehr)
}

func (ehr *ExtendedHeaderRequest) UnmarshalBinary(data []byte) error {
	if ehr == nil {
		return fmt.Errorf("header: cannot UnmarshalBinary - nil ExtendedHeader")
	}

	out, err := UnmarshalExtendedHeaderRequest(data)
	if err != nil {
		return err
	}

	*ehr = *out
	return nil
}

func (ehr *ExtendedHeaderRequest) ToProto() *header_pb.ExtendedHeaderRequest {
	return &header_pb.ExtendedHeaderRequest{
		Origin: ehr.Origin,
		Amount: ehr.Amount,
	}
}
