package header

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ipfs/go-blockservice"
	logging "github.com/ipfs/go-log/v2"

	amino "github.com/tendermint/tendermint/libs/json"
	core "github.com/tendermint/tendermint/types"

	appshares "github.com/celestiaorg/celestia-app/pkg/shares"

	"github.com/celestiaorg/celestia-app/pkg/da"

	libhead "github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/share"
)

var log = logging.Logger("header")

// ConstructFn aliases a function that creates an ExtendedHeader.
type ConstructFn = func(
	context.Context,
	*core.Block,
	*core.Commit,
	*core.ValidatorSet,
	blockservice.BlockService,
) (*ExtendedHeader, error)

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

func (eh *ExtendedHeader) New() libhead.Header {
	return new(ExtendedHeader)
}

func (eh *ExtendedHeader) IsZero() bool {
	return eh == nil
}

func (eh *ExtendedHeader) ChainID() string {
	return eh.RawHeader.ChainID
}

func (eh *ExtendedHeader) Height() int64 {
	return eh.RawHeader.Height
}

func (eh *ExtendedHeader) Time() time.Time {
	return eh.RawHeader.Time
}

var _ libhead.Header = &ExtendedHeader{}

// MakeExtendedHeader assembles new ExtendedHeader.
func MakeExtendedHeader(
	ctx context.Context,
	b *core.Block,
	comm *core.Commit,
	vals *core.ValidatorSet,
	bServ blockservice.BlockService,
) (*ExtendedHeader, error) {
	var dah DataAvailabilityHeader
	if len(b.Txs) > 0 {
		shares, err := appshares.Split(b.Data, true)
		if err != nil {
			return nil, err
		}
		extended, err := share.AddShares(ctx, appshares.ToBytes(shares), bServ)
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
	return eh, eh.Validate()
}

// Hash returns Hash of the wrapped RawHeader.
// NOTE: It purposely overrides Hash method of RawHeader to get it directly from Commit without
// recomputing.
func (eh *ExtendedHeader) Hash() libhead.Hash {
	return libhead.Hash(eh.Commit.BlockID.Hash)
}

// LastHeader returns the Hash of the last wrapped RawHeader.
func (eh *ExtendedHeader) LastHeader() libhead.Hash {
	return libhead.Hash(eh.RawHeader.LastBlockID.Hash)
}

// IsBefore returns whether the given header is of a higher height.

// Equals returns whether the hash and height of the given header match.
func (eh *ExtendedHeader) Equals(header *ExtendedHeader) bool {
	return eh.Height() == header.Height() && bytes.Equal(eh.Hash(), header.Hash())
}

// Validate performs *basic* validation to check for missed/incorrect fields.
func (eh *ExtendedHeader) Validate() error {
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

	if err := eh.ValidatorSet.VerifyCommitLight(eh.ChainID(), eh.Commit.BlockID, eh.Height(), eh.Commit); err != nil {
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
