package header

import (
	"bytes"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/light"
)

// TrustingPeriod is period through which we can trust a header's validators set.
//
// Should be significantly less than the unbonding period (e.g. unbonding
// period = 3 weeks, trusting period = 2 weeks).
//
// More specifically, trusting period + time needed to check headers + time
// needed to report and punish misbehavior should be less than the unbonding
// period.
// TODO(@Wondertan): We should request it from the network's state params
//  or listen for network params changes to always have a topical value.
var TrustingPeriod = 168 * time.Hour

// IsExpired checks if header is expired against trusting period.
func (eh *ExtendedHeader) IsExpired() bool {
	expirationTime := eh.Time.Add(TrustingPeriod)
	return !expirationTime.After(time.Now())
}

// VerifyNonAdjacent validates non-adjacent untrusted header against trusted 'eh'.
func (eh *ExtendedHeader) VerifyNonAdjacent(untrst *ExtendedHeader) error {
	if err := eh.verify(untrst); err != nil {
		return &VerifyError{Reason: err}
	}

	// Ensure that untrusted commit has enough of trusted commit's power.
	err := eh.ValidatorSet.VerifyCommitLightTrusting(eh.ChainID, untrst.Commit, light.DefaultTrustLevel)
	if err != nil {
		return &VerifyError{err}
	}

	return nil
}

// VerifyAdjacent validates adjacent untrusted header against trusted 'eh'.
func (eh *ExtendedHeader) VerifyAdjacent(untrst *ExtendedHeader) error {
	if untrst.Height != eh.Height+1 {
		return ErrNonAdjacent
	}

	if err := eh.verify(untrst); err != nil {
		return &VerifyError{Reason: err}
	}

	// Check the validator hashes are the same
	if !bytes.Equal(untrst.ValidatorsHash, eh.NextValidatorsHash) {
		return &VerifyError{
			fmt.Errorf("expected old header next validators (%X) to match those from new header (%X)",
				eh.NextValidatorsHash,
				untrst.ValidatorsHash,
			),
		}
	}

	return nil
}

// clockDrift defines how much new header's time can drift into
// the future relative to the now time during verification.
var clockDrift = 10 * time.Second

// verify performs basic verification of untrusted header.
func (eh *ExtendedHeader) verify(untrst *ExtendedHeader) error {
	if untrst.ChainID != eh.ChainID {
		return fmt.Errorf("new untrusted header has different chain %s, not %s", untrst.ChainID, eh.ChainID)
	}

	if !untrst.Time.After(eh.Time) {
		return fmt.Errorf("expected new untrusted header time %v to be after old header time %v", untrst.Time, eh.Time)
	}

	now := time.Now()
	if !untrst.Time.Before(now.Add(clockDrift)) {
		return fmt.Errorf(
			"new untrusted header has a time from the future %v (now: %v, clockDrift: %v)", untrst.Time, now, clockDrift)
	}

	return nil
}

// VerifyError is thrown on during VerifyAdjacent and VerifyNonAdjacent if verification fails.
type VerifyError struct {
	Reason error
}

func (vr *VerifyError) Error() string {
	return fmt.Sprintf("header: verify: %s", vr.Reason.Error())
}
