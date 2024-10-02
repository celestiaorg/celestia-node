package share

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"

	"github.com/celestiaorg/celestia-app/v2/pkg/da"
	"github.com/celestiaorg/rsmt2d"
)

const (
	// DataHashSize is the size of the DataHash.
	DataHashSize = 32
	// AxisRootSize is the size of the single root in AxisRoots.
	AxisRootSize = 90
)

// AxisRoots represents root commitment to multiple Shares.
// In practice, it is a commitment to all the Data in a square.
type AxisRoots = da.DataAvailabilityHeader

// DataHash is a representation of the AxisRoots hash.
type DataHash []byte

func (dh DataHash) Validate() error {
	if len(dh) != DataHashSize {
		return fmt.Errorf("invalid hash size, expected 32, got %d", len(dh))
	}
	return nil
}

func (dh DataHash) String() string {
	return fmt.Sprintf("%X", []byte(dh))
}

// IsEmptyEDS check whether DataHash corresponds to the root of an empty block EDS.
func (dh DataHash) IsEmptyEDS() bool {
	return bytes.Equal(EmptyEDSDataHash(), dh)
}

// NewSHA256Hasher returns a new instance of a SHA-256 hasher.
func NewSHA256Hasher() hash.Hash {
	return sha256.New()
}

// NewAxisRoots generates AxisRoots(DataAvailabilityHeader) using the
// provided extended data square.
func NewAxisRoots(eds *rsmt2d.ExtendedDataSquare) (*AxisRoots, error) {
	dah, err := da.NewDataAvailabilityHeader(eds)
	if err != nil {
		return nil, err
	}
	return &dah, nil
}

// RowsWithNamespace inspects the AxisRoots for the Namespace and provides
// a slices of Row indexes containing the namespace.
func RowsWithNamespace(root *AxisRoots, namespace Namespace) (idxs []int) {
	for i, row := range root.RowRoots {
		if !namespace.IsOutsideRange(row, row) {
			idxs = append(idxs, i)
		}
	}
	return
}

// RootHashForCoordinates returns the root hash for the given coordinates.
func RootHashForCoordinates(r *AxisRoots, axisType rsmt2d.Axis, rowIdx, colIdx uint) []byte {
	if axisType == rsmt2d.Row {
		return r.RowRoots[rowIdx]
	}
	return r.ColumnRoots[colIdx]
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
