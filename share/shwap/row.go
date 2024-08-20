package shwap

import (
	"bytes"
	"fmt"

	"github.com/celestiaorg/celestia-app/v2/pkg/wrapper"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// RowName is the name identifier for the row container.
const RowName = "row_v0"

// RowSide enumerates the possible sides of a row within an Extended Data Square (EDS).
type RowSide int

const (
	Left  RowSide = iota // Left side of the row.
	Right                // Right side of the row.
)

// Row represents a portion of a row in an EDS, either left or right half.
type Row struct {
	halfShares []share.Share // halfShares holds the shares of either the left or right half of a row.
	side       RowSide       // side indicates whether the row half is left or right.
}

// NewRow creates a new Row with the specified shares and side.
func NewRow(halfShares []share.Share, side RowSide) Row {
	return Row{
		halfShares: halfShares,
		side:       side,
	}
}

// RowFromEDS constructs a new Row from an Extended Data Square based on the specified index and
// side.
func RowFromShares(shares []share.Share, side RowSide) Row {
	var halfShares []share.Share
	if side == Right {
		halfShares = shares[len(shares)/2:] // Take the right half of the shares.
	} else {
		halfShares = shares[:len(shares)/2] // Take the left half of the shares.
	}

	return NewRow(halfShares, side)
}

// RowFromProto converts a protobuf Row to a Row structure.
func RowFromProto(r *pb.Row) Row {
	return Row{
		halfShares: SharesFromProto(r.SharesHalf),
		side:       sideFromProto(r.GetHalfSide()),
	}
}

// Shares reconstructs the complete row shares from the half provided, using RSMT2D for data
// recovery if needed.
func (r Row) Shares() ([]share.Share, error) {
	shares := make([]share.Share, len(r.halfShares)*2)
	offset := 0
	if r.side == Right {
		offset = len(r.halfShares) // Position the halfShares in the second half if it's the right side.
	}
	for i, share := range r.halfShares {
		shares[i+offset] = share
	}
	return share.DefaultRSMT2DCodec().Decode(shares)
}

// ToProto converts the Row to its protobuf representation.
func (r Row) ToProto() *pb.Row {
	return &pb.Row{
		SharesHalf: SharesToProto(r.halfShares),
		HalfSide:   r.side.ToProto(),
	}
}

// IsEmpty reports whether the Row is empty, i.e. doesn't contain any shares.
func (r Row) IsEmpty() bool {
	return r.halfShares == nil
}

// Validate checks if the row's shares match the expected number from the root data and validates
// the side of the row.
func (r Row) Validate(roots *share.AxisRoots, idx int) error {
	if len(r.halfShares) == 0 {
		return fmt.Errorf("empty half row")
	}
	expectedShares := len(roots.RowRoots) / 2
	if len(r.halfShares) != expectedShares {
		return fmt.Errorf("shares size doesn't match root size: %d != %d", len(r.halfShares), expectedShares)
	}
	if err := ValidateShares(r.halfShares); err != nil {
		return fmt.Errorf("invalid shares: %w", err)
	}
	if r.side != Left && r.side != Right {
		return fmt.Errorf("invalid RowSide: %d", r.side)
	}

	if err := r.verifyInclusion(roots, idx); err != nil {
		return fmt.Errorf("%w: %w", ErrFailedVerification, err)
	}
	return nil
}

// verifyInclusion verifies the integrity of the row's shares against the provided root hash for the
// given row index.
func (r Row) verifyInclusion(roots *share.AxisRoots, idx int) error {
	shrs, err := r.Shares()
	if err != nil {
		return fmt.Errorf("while extending shares: %w", err)
	}

	sqrLn := uint64(len(shrs) / 2)
	tree := wrapper.NewErasuredNamespacedMerkleTree(sqrLn, uint(idx))
	for _, s := range shrs {
		if err := tree.Push(s); err != nil {
			return fmt.Errorf("while pushing shares to NMT: %w", err)
		}
	}

	root, err := tree.Root()
	if err != nil {
		return fmt.Errorf("while computing NMT root: %w", err)
	}

	if !bytes.Equal(roots.RowRoots[idx], root) {
		return fmt.Errorf("invalid root hash: %X != %X", root, roots.RowRoots[idx])
	}
	return nil
}

// ToProto converts a RowSide to its protobuf representation.
func (s RowSide) ToProto() pb.Row_HalfSide {
	if s == Left {
		return pb.Row_LEFT
	}
	return pb.Row_RIGHT
}

// sideFromProto converts a protobuf Row_HalfSide back to a RowSide.
func sideFromProto(side pb.Row_HalfSide) RowSide {
	if side == pb.Row_LEFT {
		return Left
	}
	return Right
}
