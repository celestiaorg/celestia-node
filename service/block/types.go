package block

import (
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/service/header"

	core "github.com/celestiaorg/celestia-core/types"
)

// RawBlock is an alias to a "raw" Core block. It is "raw" because
// it is still awaiting erasure coding.
type RawBlock = core.Block

// Block represents the entirety of a Block in the Celestia network.
// It contains the erasure coded block data as well as its
// ExtendedHeader.
type Block struct {
	header *header.ExtendedHeader
	data   *ExtendedBlockData
	meta   *BlockMeta
}

//nolint:revive
// BlockMeta contains the metadata for the Block.
type BlockMeta struct {
	height     int64
	lastCommit *core.Commit
}

// ExtendedBlockData is an alias to rsmt2d's ExtendedDataSquare type.
type ExtendedBlockData = rsmt2d.ExtendedDataSquare

func (b *Block) Height() int64 {
	return b.meta.height
}

// Header returns the ExtendedHeader of the Block.
func (b *Block) Header() *header.ExtendedHeader {
	return b.header
}

// Data returns the erasure coded data of the Block.
func (b *Block) Data() *ExtendedBlockData {
	return b.data
}

// LastCommit returns the last commit of the Block.
func (b *Block) LastCommit() *core.Commit {
	return b.meta.lastCommit
}

// DataSize returns the width of the ExtendedBlockData.
func (b *Block) DataSize() uint {
	return b.data.Width()
}

// BadEncodingError contains all relevant information to
// generate a BadEncodingFraudProof.
type BadEncodingError struct{}
