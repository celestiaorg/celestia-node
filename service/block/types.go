package block

import (
	core "github.com/celestiaorg/celestia-core/types"
	"github.com/celestiaorg/rsmt2d"
)

// Raw is an alias to a "raw" Core block. It is "raw" because
// it is still awaiting erasure coding.
type Raw = core.Block

// ExtendedBlock is an alias to rsmt2d's ExtendedDataSquare type.
type ExtendedBlock = rsmt2d.ExtendedDataSquare
