package shwap

import (
	"bytes"
	"fmt"

	"github.com/celestiaorg/celestia-app/v3/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

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
	Both                 // Both sides of the row.
)

// Row represents a portion of a row in an EDS, either left or right half.
type Row struct {
	shares []libshare.Share // holds the shares of Left or Right or Both sides of the row.
	side   RowSide          // side indicates which side the shares belong to.
}

// NewRow creates a new Row with the specified shares and side.
func NewRow(shares []libshare.Share, side RowSide) Row {
	return Row{
		shares: shares,
		side:   side,
	}
}

// RowFromEDS constructs a new Row from an EDS based on the specified row index and side.
func RowFromEDS(eds *rsmt2d.ExtendedDataSquare, rowIdx int, side RowSide) (Row, error) {
	rowBytes := eds.Row(uint(rowIdx))
	shares, err := libshare.FromBytes(rowBytes)
	if err != nil {
		return Row{}, fmt.Errorf("while converting shares from bytes: %w", err)
	}

	switch side {
	case Both:
	case Left:
		shares = shares[:len(shares)/2]
	case Right:
		shares = shares[len(shares)/2:]
	default:
		return Row{}, fmt.Errorf("invalid RowSide: %d", side)
	}

	return NewRow(shares, side), nil
}

// RowFromProto converts a protobuf Row to a Row structure.
func RowFromProto(r *pb.Row) (Row, error) {
	shrs, err := SharesFromProto(r.SharesHalf)
	if err != nil {
		return Row{}, err
	}
	return Row{
		shares: shrs,
		side:   sideFromProto(r.GetHalfSide()),
	}, nil
}

// Shares reconstructs the complete row shares from the half provided, using RSMT2D for data
// recovery if needed.
// It caches the reconstructed shares for future use and converts Row to Both side.
func (r *Row) Shares() ([]libshare.Share, error) {
	if r.side == Both {
		return r.shares, nil
	}

	shares := make([]libshare.Share, len(r.shares)*2)
	offset := len(r.shares) * int(r.side)
	for i, share := range r.shares {
		shares[i+offset] = share
	}

	rowShares, err := share.DefaultRSMT2DCodec().Decode(libshare.ToBytes(shares))
	if err != nil {
		return nil, err
	}

	r.shares, err = libshare.FromBytes(rowShares)
	if err != nil {
		return nil, err
	}

	r.side = Both
	return r.shares, nil
}

// ToProto converts the Row to its protobuf representation.
func (r Row) ToProto() *pb.Row {
	if r.side == Both {
		// we don't need to send the whole row over the wire
		// so if we have both sides, we can save bandwidth and send the left half only
		return &pb.Row{
			SharesHalf: SharesToProto(r.shares[:len(r.shares)/2]),
			HalfSide:   pb.Row_LEFT,
		}
	}

	return &pb.Row{
		SharesHalf: SharesToProto(r.shares),
		HalfSide:   r.side.ToProto(),
	}
}

// IsEmpty reports whether the Row is empty, i.e. doesn't contain any shares.
func (r Row) IsEmpty() bool {
	return len(r.shares) == 0
}

// Verify checks if the row's shares match the expected number from the root data and validates
// the side of the row.
func (r *Row) Verify(roots *share.AxisRoots, idx int) error {
	if len(r.shares) == 0 {
		return fmt.Errorf("empt row")
	}
	expectedShares := len(roots.RowRoots)
	if r.side != Both {
		expectedShares /= 2
	}
	if len(r.shares) != expectedShares {
		return fmt.Errorf("shares size doesn't match root size: %d != %d", len(r.shares), expectedShares)
	}
	if r.side != Left && r.side != Right && r.side != Both {
		return fmt.Errorf("invalid RowSide: %d", r.side)
	}

	if err := r.verifyInclusion(roots, idx); err != nil {
		return fmt.Errorf("%w: %w", ErrFailedVerification, err)
	}
	return nil
}

// verifyInclusion verifies the integrity of the row's shares against the provided root hash for the
// given row index.
func (r *Row) verifyInclusion(roots *share.AxisRoots, idx int) error {
	shrs, err := r.Shares()
	if err != nil {
		return fmt.Errorf("while extending shares: %w", err)
	}

	sqrLn := uint64(len(shrs) / 2)
	tree := wrapper.NewErasuredNamespacedMerkleTree(sqrLn, uint(idx))
	for _, s := range shrs {
		if err := tree.Push(s.ToBytes()); err != nil {
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
