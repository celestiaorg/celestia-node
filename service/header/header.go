package header

import (
	"bytes"
	"context"
	"fmt"

	format "github.com/ipfs/go-ipld-format"
	bts "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/pkg/da"
	core "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-node/ipld"

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

// MakeExtendedHeader assembles new ExtendedHeader.
func MakeExtendedHeader(
	ctx context.Context,
	b *core.Block,
	comm *core.Commit,
	vals *core.ValidatorSet,
	dag format.NodeAdder,
) (*ExtendedHeader, error) {
	var dah DataAvailabilityHeader
	if len(b.Txs) > 0 {
		namespacedShares, _ := b.Data.ComputeShares()
		extended, err := ipld.AddShares(ctx, namespacedShares.RawShares(), dag)
		if err != nil {
			return nil, err
		}
		dah = da.NewDataAvailabilityHeader(extended)
	} else {
		// use MinDataAvailabilityHeader for empty block
		dah = EmptyDAH()
		log.Debugw("empty block received", "height", "blockID", "time", b.Height, b.Time.String(), comm.BlockID)
	}

	eh := &ExtendedHeader{
		RawHeader:    b.Header,
		DAH:          &dah,
		Commit:       comm,
		ValidatorSet: vals,
	}
	return eh, eh.ValidateBasic()
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

	// make sure the validator set is consistent with the header
	if valSetHash := eh.ValidatorSet.Hash(); !bytes.Equal(eh.ValidatorsHash, valSetHash) {
		return fmt.Errorf("expected validator hash of header to match validator set hash (%X != %X)",
			eh.ValidatorsHash, valSetHash,
		)
	}

	if err := eh.ValidatorSet.VerifyCommitLight(eh.ChainID, eh.Commit.BlockID, eh.Height, eh.Commit); err != nil {
		return err
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
