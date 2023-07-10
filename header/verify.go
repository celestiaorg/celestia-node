package header

import (
	"bytes"
	"fmt"
	"time"

	libhead "github.com/celestiaorg/go-header"
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

	return nil
}

// clockDrift defines how much new header's time can drift into
// the future relative to the now time during verification.
var clockDrift = 10 * time.Second

// verify performs basic verification of untrusted header.
func (eh *ExtendedHeader) verify(untrst libhead.Header) error {
	if untrst.Height() <= eh.Height() {
		return fmt.Errorf("untrusted header height(%d) <= current trusted header(%d)", untrst.Height(), eh.Height())
	}

	if untrst.ChainID() != eh.ChainID() {
		return fmt.Errorf("untrusted header has different chain %s, not %s", untrst.ChainID(), eh.ChainID())
	}

	if !untrst.Time().After(eh.Time()) {
		return fmt.Errorf("untrusted header time(%v) must be after current trusted header(%v)", untrst.Time(), eh.Time())
	}

	now := time.Now()
	if !untrst.Time().Before(now.Add(clockDrift)) {
		return fmt.Errorf(
			"new untrusted header has a time from the future %v (now: %v, clockDrift: %v)", untrst.Time(), now, clockDrift)
	}

	return nil
}
