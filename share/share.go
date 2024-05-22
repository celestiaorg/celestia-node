package share

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"
)

// DefaultRSMT2DCodec sets the default rsmt2d.Codec for shares.
var DefaultRSMT2DCodec = appconsts.DefaultCodec

const (
	// Size is a system-wide size of a share, including both data and namespace GetNamespace
	Size = appconsts.ShareSize
)

// MaxSquareSize is currently the maximum size supported for unerasured data in
// rsmt2d.ExtendedDataSquare.
var MaxSquareSize = appconsts.SquareSizeUpperBound(appconsts.LatestVersion)

// Share contains the raw share data without the corresponding namespace.
// NOTE: Alias for the byte is chosen to keep maximal compatibility, especially with rsmt2d.
// Ideally, we should define reusable type elsewhere and make everyone(Core, rsmt2d, ipld) to rely
// on it.
type Share = []byte

// GetNamespace slices Namespace out of the Share.
func GetNamespace(s Share) Namespace {
	return s[:NamespaceSize]
}

// GetData slices out data of the Share.
func GetData(s Share) []byte {
	return s[NamespaceSize:]
}

// ValidateShare checks the size of a given share.
func ValidateShare(s Share) error {
	if len(s) != Size {
		return fmt.Errorf("invalid share size: %d", len(s))
	}
	return nil
}

// ShareWithProof contains data with corresponding Merkle Proof
type ShareWithProof struct { //nolint: revive
	// Share is a full data including namespace
	Share
	// Proof is a Merkle Proof of current share
	Proof *nmt.Proof
	// Axis is a type of axis against which the share proof is computed
	Axis rsmt2d.Axis
}

// Validate validates inclusion of the share under the given root CID.
func (s *ShareWithProof) Validate(rootHash []byte, x, y, edsSize int) bool {
	isParity := x >= edsSize/2 || y >= edsSize/2
	namespace := ParitySharesNamespace
	if !isParity {
		namespace = GetNamespace(s.Share)
	}
	return s.Proof.VerifyInclusion(
		sha256.New(), // TODO(@Wondertan): This should be defined somewhere globally
		namespace.ToNMT(),
		[][]byte{s.Share},
		rootHash,
	)
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

// NewSHA256Hasher returns a new instance of a SHA-256 hasher.
func NewSHA256Hasher() hash.Hash {
	return sha256.New()
}

// RootHashForCoordinates returns the root hash for the given coordinates.
func RootHashForCoordinates(r *Root, axisType rsmt2d.Axis, rowIdx, colIdx uint) []byte {
	if axisType == rsmt2d.Row {
		return r.RowRoots[rowIdx]
	}
	return r.ColumnRoots[colIdx]
}

// FilterRootByNamespace returns the row roots from the given share.Root that contain the namespace.
// It also returns the half open range of the roots that contain the namespace.
func FilterRootByNamespace(root *Root, namespace Namespace) (rowRoots [][]byte, from, to int) {
	for i, rowRoot := range root.RowRoots {
		if !namespace.IsOutsideRange(rowRoot, rowRoot) {
			rowRoots = append(rowRoots, rowRoot)
			to = i
		}
	}
	to++
	return rowRoots, to - len(rowRoots), to
}
