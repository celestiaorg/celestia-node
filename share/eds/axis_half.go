package eds

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var codec = share.DefaultRSMT2DCodec()

// AxisHalf represents a half of data for a row or column in the EDS.
type AxisHalf struct {
	Shares []share.Share
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
func (a AxisHalf) Extended() ([]share.Share, error) {
	if a.IsParity {
		return reconstructShares(a.Shares)
	}
	return extendShares(a.Shares)
}

// extendShares constructs full axis shares from original half axis shares.
func extendShares(original []share.Share) ([]share.Share, error) {
	if len(original) == 0 {
		return nil, fmt.Errorf("original shares are empty")
	}

	parity, err := codec.Encode(original)
	if err != nil {
		return nil, fmt.Errorf("encoding: %w", err)
	}

	sqLen := len(original) * 2
	shares := make([]share.Share, sqLen)
	copy(shares, original)
	copy(shares[sqLen/2:], parity)
	return shares, nil
}

func reconstructShares(parity []share.Share) ([]share.Share, error) {
	if len(parity) == 0 {
		return nil, fmt.Errorf("parity shares are empty")
	}

	sqLen := len(parity) * 2
	shares := make([]share.Share, sqLen)
	for i := sqLen / 2; i < sqLen; i++ {
		shares[i] = parity[i-sqLen/2]
	}
	_, err := codec.Decode(shares)
	if err != nil {
		return nil, fmt.Errorf("reconstructing: %w", err)
	}
	return shares, nil
}
