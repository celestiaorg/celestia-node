package header

import (
	"bytes"
	"fmt"
	"time"
)

// Verify validates trusted header against untrusted.
// TODO(@Wondertan): Unbonding period!!!
func Verify(trusted, untrusted *ExtendedHeader) error {
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

	// Ensure that +2/3 of new validators signed correctly.
	if err := trusted.ValidatorSet.VerifyCommitLight(
		trusted.ChainID,
		untrusted.Commit.BlockID,
		untrusted.Height,
		untrusted.Commit,
	); err != nil {
		return err
	}

	return nil
}

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
	if err := untrusted.ValidatorSet.VerifyCommitLight(
		trusted.ChainID,
		untrusted.Commit.BlockID,
		untrusted.Height,
		untrusted.Commit,
	); err != nil {
		return err
	}

	return nil
}
