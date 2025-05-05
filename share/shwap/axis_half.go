package shwap

import (
	"fmt"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share"
)

var codec = share.DefaultRSMT2DCodec()

// AxisHalf represents a half of data for a row or column in the EDS.
type AxisHalf struct {
	Shares []libshare.Share
	// IsParity indicates whether the half is parity or data.
	IsParity bool
}

// ToRow converts the AxisHalf to a shwap.Row.
func (a AxisHalf) ToRow() Row {
	side := Left
	if a.IsParity {
		side = Right
	}
	return NewRow(a.Shares, side)
}

// Extended returns full axis shares from half axis shares.
func (a AxisHalf) Extended() ([]libshare.Share, error) {
	if a.IsParity {
		return reconstructShares(a.Shares)
	}
	return extendShares(a.Shares)
}

// extendShares constructs full axis shares from original half axis shares.
func extendShares(original []libshare.Share) ([]libshare.Share, error) {
	if len(original) == 0 {
		return nil, fmt.Errorf("original shares are empty")
	}

	parity, err := codec.Encode(libshare.ToBytes(original))
	if err != nil {
		return nil, fmt.Errorf("encoding: %w", err)
	}

	parityShrs, err := libshare.FromBytes(parity)
	if err != nil {
		return nil, err
	}

	sqLen := len(original) * 2
	shares := make([]libshare.Share, sqLen)
	copy(shares, original)
	copy(shares[sqLen/2:], parityShrs)
	return shares, nil
}

func reconstructShares(parity []libshare.Share) ([]libshare.Share, error) {
	if len(parity) == 0 {
		return nil, fmt.Errorf("parity shares are empty")
	}

	sqLen := len(parity) * 2
	shares := make([]libshare.Share, sqLen)
	for i := sqLen / 2; i < sqLen; i++ {
		shares[i] = parity[i-sqLen/2]
	}
	shrs, err := codec.Decode(libshare.ToBytes(shares))
	if err != nil {
		return nil, fmt.Errorf("reconstructing: %w", err)
	}

	return libshare.FromBytes(shrs)
}
