package eds

import (
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// AxisHalf represents a half of a row or column in a shwap.
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
