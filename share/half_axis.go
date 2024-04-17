package share

import (
	"bytes"
	"fmt"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	types "github.com/celestiaorg/celestia-node/share/pb"
	"github.com/celestiaorg/rsmt2d"
)

type Shares []Share

// FIXME: share.HalfAxis will be renamed to share.AxisHalf in the future after removing file.AxisHalf
type HalfAxis struct {
	Shares   []Share
	IsParity bool
}

func (ha HalfAxis) ToShares() (Shares, error) {
	shares := make(Shares, len(ha.Shares)*2)
	var from int
	if ha.IsParity {
		from = len(ha.Shares)
	}
	for i, share := range ha.Shares {
		shares[i+from] = share
	}
	shares, err := DefaultRSMT2DCodec().Decode(shares)
	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct shares: %w", err)
	}
	return shares, nil

}

func (ha HalfAxis) ToProto() *types.HalfAxis {
	return &types.HalfAxis{
		Shares:   ha.Shares,
		IsParity: ha.IsParity,
	}
}

func HalfAxisFromProto(ha *types.HalfAxis) HalfAxis {
	return HalfAxis{
		Shares:   ha.Shares,
		IsParity: ha.IsParity,
	}
}

// NewHalfAxisFromEDS constructs a new HalfAxis from the EDS.
func NewHalfAxisFromEDS(
	square *rsmt2d.ExtendedDataSquare,
	axisType rsmt2d.Axis,
	idx int,
	isParity bool,
) HalfAxis {
	sqrLn := int(square.Width())
	var shares [][]byte
	switch axisType {
	case rsmt2d.Row:
		shares = square.Row(uint(idx))
	case rsmt2d.Col:
		shares = square.Col(uint(idx))
	}

	var halfShares []Share
	if isParity {
		halfShares = shares[sqrLn/2:]
	} else {
		halfShares = shares[:sqrLn/2]
	}

	return HalfAxis{
		Shares:   halfShares,
		IsParity: isParity,
	}
}

func (shrs Shares) Validate(dah *Root, idx int) error {
	if len(shrs) == 0 {
		return fmt.Errorf("empty half row")
	}
	if len(shrs) != len(dah.RowRoots) {
		return fmt.Errorf("shares size doesn't match root size: %d != %d", len(shrs), len(dah.RowRoots))
	}

	return shrs.VerifyInclusion(dah, idx)
}

func (shrs Shares) VerifyInclusion(dah *Root, idx int) error {
	sqrLn := uint64(len(shrs) / 2)
	tree := wrapper.NewErasuredNamespacedMerkleTree(sqrLn, uint(idx))
	for _, s := range shrs {
		err := tree.Push(s)
		if err != nil {
			return fmt.Errorf("while pushing shares to NMT: %w", err)
		}
	}

	root, err := tree.Root()
	if err != nil {
		return fmt.Errorf("while computing NMT root: %w", err)
	}

	if !bytes.Equal(dah.RowRoots[idx], root) {
		return fmt.Errorf("invalid root hash: %X != %X", root, dah.RowRoots[idx])
	}
	return nil
}
