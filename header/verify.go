package header

import (
	"bytes"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/types"

	libhead "github.com/celestiaorg/go-header"
)

// clockDrift defines how much new header's time can drift into
// the future relative to the now time during verification.
var clockDrift = 10 * time.Second

// trustingPeriod is period through which we can trust a header's validators set.
//
// Should be significantly less than the unbonding period (e.g. unbonding
// period = 3 weeks, trusting period = 2 weeks).
//
// More specifically, trusting period + time needed to check headers + time
// needed to report and punish misbehavior should be less than the unbonding
// period.
var trustingPeriod = 168 * time.Hour

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
	if !isAdjacent {
		// if the header is non-adjacent, it will be verified later via
		// VerifyAdjacent once its adjacent (n-1) header is synced
		return nil
	}

	// perform light verification on adjacent headers
	trustedSigned := &types.SignedHeader{
		Header: &eh.RawHeader,
		Commit: eh.Commit,
	}
	untrustedSigned := &types.SignedHeader{
		Header: &untrst.RawHeader,
		Commit: untrst.Commit,
	}
	if err := light.VerifyAdjacent(
		trustedSigned,
		untrustedSigned, untrst.ValidatorSet,
		trustingPeriod, time.Now(), clockDrift,
	); err != nil {
		return err
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
