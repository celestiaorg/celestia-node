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
func IsExpired(eh *ExtendedHeader) bool {
	expirationTime := eh.Time.Add(TrustingPeriod)
	return !expirationTime.After(time.Now())
}

// VerifyNonAdjacent validates non-adjacent 'untrusted' header against 'trusted'.
func VerifyNonAdjacent(trusted, untrusted *ExtendedHeader) error {
	if untrusted.ChainID != trusted.ChainID {
		return fmt.Errorf("header belongs to another chain %q, not %q", untrusted.ChainID, trusted.ChainID)
	}

	if !untrusted.Time.After(trusted.Time) {
		return fmt.Errorf("expected new header time %v to be after old header time %v",
			untrusted.Time,
			trusted.Time)
	}

	now := time.Now()
	if !untrusted.Time.Before(now) {
		return fmt.Errorf("new header has a time from the future %v (now: %v)",
			untrusted.Time,
			now)
	}

	// Ensure that untrusted commit has enough of trusted commit's power.
	if err := trusted.ValidatorSet.VerifyCommitLightTrusting(
		trusted.ChainID,
		untrusted.Commit,
		light.DefaultTrustLevel, // default Tendermint value
	); err != nil {
		return err
	}

	return nil
}

// VerifyAdjacent validates adjacent 'untrusted' header against 'trusted'.
func VerifyAdjacent(trusted, untrusted *ExtendedHeader) error {
	if untrusted.Height != trusted.Height+1 {
		return fmt.Errorf("headers must be adjacent in height")
	}

	if untrusted.ChainID != trusted.ChainID {
		return fmt.Errorf("header belongs to another chain %q, not %q", untrusted.ChainID, trusted.ChainID)
	}

	if !untrusted.Time.After(trusted.Time) {
		return fmt.Errorf("expected new header time %v to be after old header time %v",
			untrusted.Time,
			trusted.Time)
	}

	now := time.Now()
	if !untrusted.Time.Before(now) {
		return fmt.Errorf("new header has a time from the future %v (now: %v)",
			untrusted.Time,
			now)
	}

	// Check the validator hashes are the same
	if !bytes.Equal(untrusted.ValidatorsHash, trusted.NextValidatorsHash) {
		return fmt.Errorf("expected old header next validators (%X) to match those from new header (%X)",
			trusted.NextValidatorsHash,
			untrusted.ValidatorsHash,
		)
	}

	// Ensure that +2/3 of new validators signed correctly.
	return untrusted.ValidatorSet.VerifyCommitLight(
		trusted.ChainID,
		untrusted.Commit.BlockID,
		untrusted.Height,
		untrusted.Commit,
	)
}
