package share

import (
	"fmt"
	types "github.com/celestiaorg/celestia-node/share/pb"
	"github.com/celestiaorg/rsmt2d"
)

type side int

const (
	Left side = iota
	Right
)

type Row struct {
	halfShares Shares
	side       side
}

func NewRow(halfShares []Share, side side) Row {
	return Row{
		halfShares: halfShares,
		side:       side,
	}
}

func (r Row) Shares() (Shares, error) {
	shares := make(Shares, len(r.halfShares)*2)
	from := 0
	// If it's Right half, we need to append to the second half of the shares.
	if r.side == Right {
		from = len(r.halfShares)
	}
	for i, share := range r.halfShares {
		shares[i+from] = share
	}
	shares, err := DefaultRSMT2DCodec().Decode(shares)
	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct shares: %w", err)
	}
	return shares, nil

}

func (r Row) ToProto() *types.Row {
	return &types.Row{
		SharesHalf: Shares(r.halfShares).ToProto(),
		HalfSide:   r.side.ToProto(),
	}
}

func RowFromProto(r *types.Row) Row {
	return Row{
		halfShares: SharesFromProto(r.SharesHalf),
		side:       sideFromProto(r.GetHalfSide()),
	}
}

// NewRowFromEDS constructs a new Row from the EDS.
func NewRowFromEDS(
	square *rsmt2d.ExtendedDataSquare,
	idx int,
	side side,
) Row {
	sqrLn := int(square.Width())
	shares := square.Row(uint(idx))
	var halfShares []Share
	if side == Right {
		halfShares = shares[sqrLn/2:]
	} else {
		halfShares = shares[:sqrLn/2]
	}

	return Row{
		halfShares: halfShares,
		side:       side,
	}
}

func (s side) ToProto() types.Row_HalfSide {
	if s == Left {
		return types.Row_LEFT
	}
	return types.Row_RIGHT
}

func sideFromProto(side types.Row_HalfSide) side {
	if side == types.Row_LEFT {
		return Left
	}
	return Right
}
