package header

import (
	"fmt"
)

type ErrValidatorHashMismatch struct {
	Reason error
}

func (e *ErrValidatorHashMismatch) Error() string {
	return fmt.Sprintf("validator hash mismatch error: %s", e.Reason.Error())
}

type ErrLastHeaderHashMismatch struct {
	Reason error
}

func (e *ErrLastHeaderHashMismatch) Error() string {
	return fmt.Sprintf("last header hash mismatch error: %s", e.Reason.Error())
}
