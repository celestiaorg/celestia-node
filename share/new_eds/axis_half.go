package eds

import (
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

type AxisHalf struct {
	Shares   []share.Share
	IsParity bool
}

func (a AxisHalf) ToRow() shwap.Row {
	side := shwap.Left
	if a.IsParity {
		side = shwap.Right
	}
	return shwap.NewRow(a.Shares, side)
}
