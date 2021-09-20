package block

import (
	"github.com/celestiaorg/rsmt2d"

	core "github.com/celestiaorg/celestia-core/types"
)

// Raw is an alias to a "raw" Core block. It is "raw" because
// it is still awaiting erasure coding.
type Raw = core.Block

// ExtendedBlock contains the erasure coded block data as well as its
// ExtendedHeader.
type ExtendedBlock struct {
	// TODO @renaynay: include pointer to ExtendedHeader
	data *ExtendedBlockData
}

// ExtendedBlockData is an alias to rsmt2d's ExtendedDataSquare type.
type ExtendedBlockData = rsmt2d.ExtendedDataSquare

func (e *ExtendedBlock) Data() ExtendedBlockData {
	return *e.data
}
