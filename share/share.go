package share

import (
	"github.com/celestiaorg/celestia-app/v6/pkg/appconsts"
)

// DefaultRSMT2DCodec sets the default rsmt2d.Codec for shares.
var DefaultRSMT2DCodec = appconsts.DefaultCodec

// MaxSquareSize is currently the maximum size supported for unerasured data in
// rsmt2d.ExtendedDataSquare.
var MaxSquareSize = appconsts.SquareSizeUpperBound
