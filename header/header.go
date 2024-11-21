package header

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/light"
	core "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/v3/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v3/pkg/da"
	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/rsmt2d"
)

// ConstructFn aliases a function that creates an ExtendedHeader.
type ConstructFn = func(
	*core.Header,
	*core.Commit,
	*core.ValidatorSet,
	*rsmt2d.ExtendedDataSquare,
) (*ExtendedHeader, error)

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

// MakeExtendedHeader assembles new ExtendedHeader.
func MakeExtendedHeader(
	h *core.Header,
	comm *core.Commit,
	vals *core.ValidatorSet,
	eds *rsmt2d.ExtendedDataSquare,
) (*ExtendedHeader, error) {
	var (
		dah da.DataAvailabilityHeader
		err error
	)
	switch eds {
	case nil:
		dah = da.MinDataAvailabilityHeader()
	default:
		dah, err = da.NewDataAvailabilityHeader(eds)
		if err != nil {
			return nil, err
		}
	}

	eh := &ExtendedHeader{
		RawHeader:    *h,
		DAH:          &dah,
		Commit:       comm,
		ValidatorSet: vals,
	}
	return eh, nil
}

func (eh *ExtendedHeader) New() *ExtendedHeader {
	return new(ExtendedHeader)
}

func (eh *ExtendedHeader) IsZero() bool {
	return eh == nil
}

func (eh *ExtendedHeader) ChainID() string {
	return eh.RawHeader.ChainID
}

func (eh *ExtendedHeader) Height() uint64 {
	return uint64(eh.RawHeader.Height)
}

func (eh *ExtendedHeader) Time() time.Time {
	return eh.RawHeader.Time
}

// Hash returns Hash of the wrapped RawHeader.
// NOTE: It purposely overrides Hash method of RawHeader to get it directly from Commit without
// recomputing.
func (eh *ExtendedHeader) Hash() libhead.Hash {
	return eh.Commit.BlockID.Hash.Bytes()
}

// LastHeader returns the Hash of the last wrapped RawHeader.
func (eh *ExtendedHeader) LastHeader() libhead.Hash {
	return libhead.Hash(eh.RawHeader.LastBlockID.Hash)
}

// Equals returns whether the hash and height of the given header match.
func (eh *ExtendedHeader) Equals(header *ExtendedHeader) bool {
	return eh.Height() == header.Height() && bytes.Equal(eh.Hash(), header.Hash())
}

// Validate performs *basic* validation to check for missed/incorrect fields.
func (eh *ExtendedHeader) Validate() error {
	err := eh.RawHeader.ValidateBasic()
	if err != nil {
		return fmt.Errorf("ValidateBasic error on RawHeader at height %d: %w", eh.Height(), err)
	}

	if eh.RawHeader.Version.App == 0 || eh.RawHeader.Version.App > appconsts.LatestVersion {
		return fmt.Errorf("header received at height %d has version %d, this node supports up "+
			"to version %d. Please upgrade to support new version. Note, 0 is not a valid version",
			eh.RawHeader.Height, eh.RawHeader.Version.App, appconsts.LatestVersion)
	}

	err = eh.Commit.ValidateBasic()
	if err != nil {
		return fmt.Errorf("ValidateBasic error on Commit at height %d: %w", eh.Height(), err)
	}

	err = eh.ValidatorSet.ValidateBasic()
	if err != nil {
		return fmt.Errorf("ValidateBasic error on ValidatorSet at height %d: %w", eh.Height(), err)
	}

	// make sure the validator set is consistent with the header
	if valSetHash := eh.ValidatorSet.Hash(); !bytes.Equal(eh.ValidatorsHash, valSetHash) {
		return fmt.Errorf("expected validator hash of header to match validator set hash (%X != %X) at height %d",
			eh.ValidatorsHash, valSetHash, eh.Height(),
		)
	}

	// ensure data root from raw header matches computed root
	if !bytes.Equal(eh.DAH.Hash(), eh.DataHash) {
		return fmt.Errorf("mismatch between data hash commitment from core header and computed data root "+
			"at height %d: data hash: %X, computed root: %X", eh.Height(), eh.DataHash, eh.DAH.Hash())
	}

	// Make sure the header is consistent with the commit.
	if eh.Commit.Height != eh.RawHeader.Height {
		return fmt.Errorf("header and commit height mismatch: %d vs %d", eh.RawHeader.Height, eh.Commit.Height)
	}
	if hhash, chash := eh.RawHeader.Hash(), eh.Commit.BlockID.Hash; !bytes.Equal(hhash, chash) {
		return fmt.Errorf("commit signs block %X, header is block %X", chash, hhash)
	}

	err = eh.ValidatorSet.VerifyCommitLight(eh.ChainID(), eh.Commit.BlockID, int64(eh.Height()), eh.Commit)
	if err != nil {
		return fmt.Errorf("VerifyCommitLight error at height %d: %w", eh.Height(), err)
	}

	err = eh.DAH.ValidateBasic()
	if err != nil {
		return fmt.Errorf("ValidateBasic error on DAH at height %d: %w", eh.RawHeader.Height, err)
	}
	return nil
}

var (
	ErrValidatorHashMismatch           = errors.New("validator hash mismatch")
	ErrLastHeaderHashMismatch          = errors.New("last header hash mismatch")
	ErrVerifyCommitLightTrustingFailed = errors.New("commit light trusting verification failed")
)

// Verify validates given untrusted Header against trusted ExtendedHeader.
func (eh *ExtendedHeader) Verify(untrst *ExtendedHeader) error {
	isAdjacent := eh.Height()+1 == untrst.Height()
	if isAdjacent {
		// Optimized verification for adjacent headers
		// Check the validator hashes are the same
		if !bytes.Equal(untrst.ValidatorsHash, eh.NextValidatorsHash) {
			return &libhead.VerifyError{
				Reason: fmt.Errorf("%w: expected (%X), but got (%X)",
					ErrValidatorHashMismatch,
					eh.NextValidatorsHash,
					untrst.ValidatorsHash,
				),
			}
		}

		if !bytes.Equal(untrst.LastHeader(), eh.Hash()) {
			return &libhead.VerifyError{
				Reason: fmt.Errorf("%w: expected (%X), but got (%X)",
					ErrLastHeaderHashMismatch,
					eh.Hash(),
					untrst.LastHeader(),
				),
			}
		}

		return nil
	}

	if err := eh.ValidatorSet.VerifyCommitLightTrusting(eh.ChainID(), untrst.Commit, light.DefaultTrustLevel); err != nil {
		return &libhead.VerifyError{
			Reason:      fmt.Errorf("%w: %w", ErrVerifyCommitLightTrustingFailed, err),
			SoftFailure: core.IsErrNotEnoughVotingPowerSigned(err),
		}
	}
	return nil
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

// MarshalJSON marshals an ExtendedHeader to JSON.
// Uses tendermint encoder for tendermint compatibility.
func (eh *ExtendedHeader) MarshalJSON() ([]byte, error) {
	// alias the type to avoid going into recursion loop
	// because tmjson.Marshal invokes custom json marshaling
	type Alias ExtendedHeader
	return tmjson.Marshal((*Alias)(eh))
}

// UnmarshalJSON unmarshals an ExtendedHeader from JSON.
// Uses tendermint decoder for tendermint compatibility.
func (eh *ExtendedHeader) UnmarshalJSON(data []byte) error {
	// alias the type to avoid going into recursion loop
	// because tmjson.Unmarshal invokes custom json unmarshalling
	type Alias ExtendedHeader
	return tmjson.Unmarshal(data, (*Alias)(eh))
}

var _ libhead.Header[*ExtendedHeader] = &ExtendedHeader{}
