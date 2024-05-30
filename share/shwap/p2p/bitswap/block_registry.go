package bitswap

import (
	"hash"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// RegisterBlock registers the new Block type and multihash for it.
func RegisterBlock(mhcode, codec uint64, size int, bldrFn func(cid.Cid) (Block, error)) {
	mh.Register(mhcode, func() hash.Hash {
		return &hasher{IDSize: size}
	})
	specRegistry[mhcode] = blockSpec{
		size:    size,
		codec:   codec,
		builder: bldrFn,
	}
}

// blockSpec holds
type blockSpec struct {
	size    int
	codec   uint64
	builder func(cid.Cid) (Block, error)
}

var specRegistry = make(map[uint64]blockSpec)
