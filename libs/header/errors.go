package header

import "fmt"

// VerifyError is thrown on during VerifyAdjacent and VerifyNonAdjacent if verification fails.
type VerifyError struct {
	Reason error
}

func (vr *VerifyError) Error() string {
	return fmt.Sprintf("header: verify: %s", vr.Reason.Error())
}
