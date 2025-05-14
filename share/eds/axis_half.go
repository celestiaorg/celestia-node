package eds

import (
	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// AxisHalf represents a half of data for a row or column in the EDS.
type AxisHalf struct {
	Shares []libshare.Share
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
func (a AxisHalf) Extended() ([]libshare.Share, error) {
	if a.IsParity {
		return share.ReconstructShares(a.Shares)
	}
	return share.ExtendShares(a.Shares)
}
