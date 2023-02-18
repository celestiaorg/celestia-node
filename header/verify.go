package header

import (
	"bytes"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/light"

	libhead "github.com/celestiaorg/celestia-node/libs/header"
)

// Verify validates given untrusted Header against trusted ExtendedHeader.
func (eh *ExtendedHeader) Verify(untrusted libhead.Header) error {
	untrst, ok := untrusted.(*ExtendedHeader)
	if !ok {
		// if the header of the type was given, something very wrong happens
		panic(fmt.Sprintf("invalid header type: expected %T, got %T", eh, untrusted))
	}

	if err := eh.verify(untrst); err != nil {
		return &libhead.VerifyError{Reason: err}
	}

	isAdjacent := eh.Height()+1 == untrst.Height()
	if isAdjacent {
		// Optimized verification for adjacent headers
		// Check the validator hashes are the same
		if !bytes.Equal(untrst.ValidatorsHash, eh.NextValidatorsHash) {
			return &libhead.VerifyError{
				Reason: fmt.Errorf("expected old header next validators (%X) to match those from new header (%X)",
					eh.NextValidatorsHash,
					untrst.ValidatorsHash,
				),
			}
		}

		if !bytes.Equal(untrst.LastHeader(), eh.Hash()) {
			return &libhead.VerifyError{
				Reason: fmt.Errorf("expected new header to point to last header hash (%X), but got %X)",
					eh.Hash(),
					untrst.LastHeader(),
				),
			}
		}

		return nil
	}

	// Ensure that untrusted commit has enough of trusted commit's power.
	err := eh.ValidatorSet.VerifyCommitLightTrusting(eh.ChainID(), untrst.Commit, light.DefaultTrustLevel)
	if err != nil {
		return &libhead.VerifyError{Reason: err}
	}

	return nil
}

// clockDrift defines how much new header's time can drift into
// the future relative to the now time during verification.
var clockDrift = 10 * time.Second

// verify performs basic verification of untrusted header.
func (eh *ExtendedHeader) verify(untrst *ExtendedHeader) error {
	if untrst.ChainID() != eh.ChainID() {
		return fmt.Errorf("new untrusted header has different chain %s, not %s", untrst.ChainID(), eh.ChainID())
	}

	if !untrst.Time().After(eh.Time()) {
		return fmt.Errorf("expected new untrusted header time %v to be after old header time %v", untrst.Time(), eh.Time())
	}

	now := time.Now()
	if !untrst.Time().Before(now.Add(clockDrift)) {
		return fmt.Errorf(
			"new untrusted header has a time from the future %v (now: %v, clockDrift: %v)", untrst.Time(), now, clockDrift)
	}

	return nil
}
