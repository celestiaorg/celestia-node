package eds

import (
	"fmt"

	gosquare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var codec = share.DefaultRSMT2DCodec()

// AxisHalf represents a half of data for a row or column in the EDS.
type AxisHalf struct {
	Shares []gosquare.Share
	// IsParity indicates whether the half is parity or data.
	IsParity bool
}

// ToRow converts the AxisHalf to a shwap.Row.
func (a AxisHalf) ToRow() shwap.Row {
	side := shwap.Left
	if a.IsParity {
		side = shwap.Right
	}
	return shwap.NewRow(a.Shares, side)
}

// Extended returns full axis shares from half axis shares.
func (a AxisHalf) Extended() ([]gosquare.Share, error) {
	if a.IsParity {
		return reconstructShares(a.Shares)
	}
	return extendShares(a.Shares)
}

// extendShares constructs full axis shares from original half axis shares.
func extendShares(original []gosquare.Share) ([]gosquare.Share, error) {
	if len(original) == 0 {
		return nil, fmt.Errorf("original shares are empty")
	}

	parity, err := codec.Encode(gosquare.ToBytes(original))
	if err != nil {
		return nil, fmt.Errorf("encoding: %w", err)
	}

	parityShrs, err := gosquare.FromBytes(parity)
	if err != nil {
		return nil, err
	}

	sqLen := len(original) * 2
	shares := make([]gosquare.Share, sqLen)
	copy(shares, original)
	copy(shares[sqLen/2:], parityShrs)
	return shares, nil
}

func reconstructShares(parity []gosquare.Share) ([]gosquare.Share, error) {
	if len(parity) == 0 {
		return nil, fmt.Errorf("parity shares are empty")
	}

	sqLen := len(parity) * 2
	shares := make([]gosquare.Share, sqLen)
	for i := sqLen / 2; i < sqLen; i++ {
		shares[i] = parity[i-sqLen/2]
	}
	shrs, err := codec.Decode(gosquare.ToBytes(shares))
	if err != nil {
		return nil, fmt.Errorf("reconstructing: %w", err)
	}

	return gosquare.FromBytes(shrs)
}
