package block

import (
	core "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/rsmt2d"
)

// Block represents the entirety of a Block in the Celestia network.
// It contains the erasure coded block data as well as its
// ExtendedHeader.
type Block struct {
	header *header.ExtendedHeader
	data   *ExtendedBlockData
}

// ExtendedBlockData is an alias to rsmt2d's ExtendedDataSquare type.
type ExtendedBlockData = rsmt2d.ExtendedDataSquare

// Header returns the ExtendedHeader of the Block.
func (b *Block) Header() *header.ExtendedHeader {
	return b.header
}

// Data returns the erasure coded data of the Block.
func (b *Block) Data() *ExtendedBlockData {
	return b.data
}

// Commit returns the commit of the Block.
func (b *Block) Commit() *core.Commit {
	return b.header.Commit
}

// DataSize returns the width of the ExtendedBlockData.
func (b *Block) DataSize() uint {
	return b.data.Width()
}
