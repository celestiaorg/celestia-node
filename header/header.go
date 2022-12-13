package header

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	bts "github.com/tendermint/tendermint/libs/bytes"
	amino "github.com/tendermint/tendermint/libs/json"
	core "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/da"
	appshares "github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

var log = logging.Logger("header")

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

type StoreFn = func(ctx context.Context, root []byte, square *rsmt2d.ExtendedDataSquare) error

// MakeExtendedHeader assembles new ExtendedHeader.
func MakeExtendedHeader(
	ctx context.Context,
	b *core.Block,
	comm *core.Commit,
	vals *core.ValidatorSet,
	store StoreFn,
) (*ExtendedHeader, error) {
	var (
		dah DataAvailabilityHeader
		err error
	)
	if len(b.Txs) > 0 {
		// extend and store block data
		var eds *rsmt2d.ExtendedDataSquare
		dah, eds, err = extendData(b.Data)
		if err != nil {
			return nil, err
		}
		err = store(ctx, dah.Hash(), eds)
		if err != nil {
			return nil, err
		}
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

// extendAndStoreData extends the given block data and stores it in the
// given eds.Store, returning the resulting DataAvailabilityHeader.
func extendData(data core.Data) (da.DataAvailabilityHeader, *rsmt2d.ExtendedDataSquare, error) {
	shares, err := appshares.Split(data, true)
	if err != nil {
		return da.DataAvailabilityHeader{}, nil, err
	}
	size := utils.SquareSize(len(shares))
	// flatten shares // TODO @renaynay: I wish appshares.Split returned shares as [][]byte instead of the alias.
	flattened := make([][]byte, len(shares))
	for i, share := range shares {
		copy(flattened[i], share)
	}
	eds, err := da.ExtendShares(size, flattened)
	if err != nil {
		return da.DataAvailabilityHeader{}, nil, err
	}
	// generate dah
	dah := da.NewDataAvailabilityHeader(eds)
	return dah, eds, nil
}

// Hash returns Hash of the wrapped RawHeader.
// NOTE: It purposely overrides Hash method of RawHeader to get it directly from Commit without
// recomputing.
func (eh *ExtendedHeader) Hash() bts.HexBytes {
	return eh.Commit.BlockID.Hash
}

// LastHeader returns the Hash of the last wrapped RawHeader.
func (eh *ExtendedHeader) LastHeader() bts.HexBytes {
	return eh.RawHeader.LastBlockID.Hash
}

// IsBefore returns whether the given header is of a higher height.
func (eh *ExtendedHeader) IsBefore(h *ExtendedHeader) bool {
	return eh.Height < h.Height
}

// Equals returns whether the hash and height of the given header match.
func (eh *ExtendedHeader) Equals(header *ExtendedHeader) bool {
	return eh.Height == header.Height && bytes.Equal(eh.Hash(), header.Hash())
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

	// ensure data root from raw header matches computed root
	if !bytes.Equal(eh.DAH.Hash(), eh.DataHash) {
		return fmt.Errorf("mismatch between data hash commitment from core header and computed data root: "+
			"data hash: %X, computed root: %X", eh.DataHash, eh.DAH.Hash())
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

// MarshalJSON marshals an ExtendedHeader to JSON. The ValidatorSet is wrapped with amino encoding,
// to be able to unmarshal the crypto.PubKey type back from JSON.
func (eh *ExtendedHeader) MarshalJSON() ([]byte, error) {
	type Alias ExtendedHeader
	validatorSet, err := amino.Marshal(eh.ValidatorSet)
	if err != nil {
		return nil, err
	}
	return json.Marshal(&struct {
		ValidatorSet json.RawMessage `json:"validator_set"`
		*Alias
	}{
		ValidatorSet: validatorSet,
		Alias:        (*Alias)(eh),
	})
}

// UnmarshalJSON unmarshals an ExtendedHeader from JSON. The ValidatorSet is wrapped with amino
// encoding, to be able to unmarshal the crypto.PubKey type back from JSON.
func (eh *ExtendedHeader) UnmarshalJSON(data []byte) error {
	type Alias ExtendedHeader
	aux := &struct {
		ValidatorSet json.RawMessage `json:"validator_set"`
		*Alias
	}{
		Alias: (*Alias)(eh),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	valSet := new(core.ValidatorSet)
	if err := amino.Unmarshal(aux.ValidatorSet, valSet); err != nil {
		return err
	}

	eh.ValidatorSet = valSet
	return nil
}
