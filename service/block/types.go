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
// TODO @renaynay: ExtendedBlock still needs to be spec'd out.
type ExtendedBlock struct {
	// TODO @renaynay: include pointer to ExtendedHeader
	data *ExtendedBlockData
}

// ExtendedBlockData is an alias to rsmt2d's ExtendedDataSquare type.
// TODO @renaynay: ExtendedBlockData still needs to be spec'd out.
type ExtendedBlockData = rsmt2d.ExtendedDataSquare

func (e *ExtendedBlock) Data() *ExtendedBlockData {
	return e.data
}

// BadEncodingError contains all relevant information to
// generate a BadEncodingFraudProof.
// TODO @renaynay: BadEncodingError still needs to be spec'd out.
type BadEncodingError struct{}
