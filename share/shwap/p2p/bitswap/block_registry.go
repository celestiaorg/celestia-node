package bitswap

import (
	"fmt"
	"hash"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// registerBlock registers the new Block type and multihash for it.
func registerBlock(mhcode, codec uint64, size int, bldrFn func(cid.Cid) (Block, error)) {
	mh.Register(mhcode, func() hash.Hash {
		return &hasher{IDSize: size}
	})
	specRegistry[mhcode] = blockSpec{
		size:    size,
		codec:   codec,
		builder: bldrFn,
	}
}

// blockSpec holds constant metadata about particular Block types.
type blockSpec struct {
	size    int
	codec   uint64
	builder func(cid.Cid) (Block, error)
}

func (spec *blockSpec) String() string {
	return fmt.Sprintf("BlockSpec{size: %d, codec: %d}", spec.size, spec.codec)
}

var specRegistry = make(map[uint64]blockSpec)
