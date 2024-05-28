package bitswap

import (
	"hash"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/celestiaorg/rsmt2d"
)

// RegisterBlock registers the new Block type.
func RegisterBlock(mhcode, codec uint64, size int, bldrFn func(cid.Cid) (blockBuilder, error)) {
	mh.Register(mhcode, func() hash.Hash {
		return &hasher{IDSize: size}
	})
	specRegistry[mhcode] = idSpec{
		size:    size,
		codec:   codec,
		builder: bldrFn,
	}
}

var specRegistry = make(map[uint64]idSpec)

type idSpec struct {
	size    int
	codec   uint64
	builder func(cid.Cid) (blockBuilder, error)
}

type blockBuilder interface {
	// BlockFromEDS gets Bitswap Block out of the EDS.
	BlockFromEDS(*rsmt2d.ExtendedDataSquare) (blocks.Block, error)
}
