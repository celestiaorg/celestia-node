package header

import (
	"fmt"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

// ErrDataRootMismatch represents an error that occurs when a
// RawHeader contains a DataRoot that does not match the computed
// data root upon ExtendedHeader validation.
type ErrDataRootMismatch struct {
	Height       uint64
	ComputedRoot tmbytes.HexBytes
	DataRoot     tmbytes.HexBytes
}

func (e *ErrDataRootMismatch) Error() string {
	return fmt.Sprintf("mismatch between data hash commitment from core header"+
		" and computed data root at height %d: data hash: %X, computed root: %X",
		e.Height, e.DataRoot, e.ComputedRoot)
}
