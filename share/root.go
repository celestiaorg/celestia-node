package share

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/rsmt2d"
)

// Root represents root commitment to multiple Shares.
// In practice, it is a commitment to all the Data in a square.
type Root = da.DataAvailabilityHeader

// NewRoot generates Root(DataAvailabilityHeader) using the
// provided extended data square.
func NewRoot(eds *rsmt2d.ExtendedDataSquare) (*Root, error) {
	dah, err := da.NewDataAvailabilityHeader(eds)
	if err != nil {
		return nil, err
	}
	return &dah, nil
}

// DataHash is a representation of the Root hash.
type DataHash []byte

func (dh DataHash) Validate() error {
	if len(dh) != 32 {
		return fmt.Errorf("invalid hash size, expected 32, got %d", len(dh))
	}
	return nil
}

func (dh DataHash) String() string {
	return fmt.Sprintf("%X", []byte(dh))
}

// IsEmptyRoot check whether DataHash corresponds to the root of an empty block EDS.
func (dh DataHash) IsEmptyRoot() bool {
	return bytes.Equal(EmptyRoot().Hash(), dh)
}

// MustDataHashFromString converts a hex string to a valid datahash.
func MustDataHashFromString(datahash string) DataHash {
	dh, err := hex.DecodeString(datahash)
	if err != nil {
		panic(fmt.Sprintf("datahash conversion: passed string was not valid hex: %s", datahash))
	}
	err = DataHash(dh).Validate()
	if err != nil {
		panic(fmt.Sprintf("datahash validation: passed hex string failed: %s", err))
	}
	return dh
}

func rootHashForCoordinates(r *Root, axisType rsmt2d.Axis, x, y uint) []byte {
	if axisType == rsmt2d.Row {
		return r.RowRoots[y]
	}
	return r.ColumnRoots[x]
}
