package share

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"hash"

	"github.com/celestiaorg/celestia-app/v6/pkg/da"
	libshare "github.com/celestiaorg/go-square/v3/share"
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
func RowsWithNamespace(root *AxisRoots, namespace libshare.Namespace) (idxs []int, err error) {
	for i, row := range root.RowRoots {
		outside, err := IsOutsideRange(namespace, row, row)
		if err != nil {
			return nil, err
		}
		if !outside {
			idxs = append(idxs, i)
		}
	}
	return idxs, nil
}

// RootHashForCoordinates returns the root hash for the given coordinates.
func RootHashForCoordinates(r *AxisRoots, axisType rsmt2d.Axis, rowIdx, colIdx uint) []byte {
	if axisType == rsmt2d.Row {
		return r.RowRoots[rowIdx]
	}
	return r.ColumnRoots[colIdx]
}

// IsOutsideRange checks if the namespace is outside the min-max range of the given hashes.
func IsOutsideRange(namespace libshare.Namespace, leftHash, rightHash []byte) (bool, error) {
	if len(leftHash) < libshare.NamespaceSize {
		return false, fmt.Errorf("left can't be less than %d", libshare.NamespaceSize)
	}
	if len(rightHash) < 2*libshare.NamespaceSize {
		return false, fmt.Errorf("rightHash can't be less than %d", 2*libshare.NamespaceSize)
	}
	ns1, err := libshare.NewNamespaceFromBytes(leftHash[:libshare.NamespaceSize])
	if err != nil {
		return false, err
	}
	ns2, err := libshare.NewNamespaceFromBytes(rightHash[libshare.NamespaceSize : libshare.NamespaceSize*2])
	if err != nil {
		return false, err
	}
	return namespace.IsLessThan(ns1) || !namespace.IsLessOrEqualThan(ns2), nil
}

// IsAboveMax checks if the namespace is above the maximum namespace of the given hash.
func IsAboveMax(namespace libshare.Namespace, hash []byte) (bool, error) {
	if len(hash) < 2*libshare.NamespaceSize {
		return false, fmt.Errorf("hash can't be less than: %d", 2*libshare.NamespaceSize)
	}
	ns, err := libshare.NewNamespaceFromBytes(hash[libshare.NamespaceSize : libshare.NamespaceSize*2])
	if err != nil {
		return false, err
	}
	return !namespace.IsLessOrEqualThan(ns), nil
}

// IsBelowMin checks if the target namespace is below the minimum namespace of the given hash.
func IsBelowMin(namespace libshare.Namespace, hash []byte) (bool, error) {
	if len(hash) < libshare.NamespaceSize {
		return false, fmt.Errorf("hash can't be less than: %d", libshare.NamespaceSize)
	}
	ns1, err := libshare.NewNamespaceFromBytes(hash[:libshare.NamespaceSize])
	if err != nil {
		return false, err
	}
	return namespace.IsLessThan(ns1), nil
}
