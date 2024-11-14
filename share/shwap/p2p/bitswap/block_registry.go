package bitswap

import (
	"fmt"
	"hash"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// EmptyBlock constructs an empty Block with type in the given CID.
func EmptyBlock(cid cid.Cid) (Block, error) {
	spec, ok := specRegistry[cid.Prefix().MhType]
	if !ok {
		return nil, fmt.Errorf("unsupported Block type: %v", cid.Prefix().MhType)
	}

	blk, err := spec.builder(cid)
	if err != nil {
		return nil, fmt.Errorf("failed to build a Block for %s: %w", spec.String(), err)
	}

	return blk, nil
}

// maxBlockSize returns the maximum size of the Block type in the given CID.
func maxBlockSize(cid cid.Cid) (int, error) {
	spec, ok := specRegistry[cid.Prefix().MhType]
	if !ok {
		return -1, fmt.Errorf("unsupported Block type: %v", cid.Prefix().MhType)
	}

	return spec.maxSize, nil
}

// registerBlock registers the new Block type and multihash for it.
func registerBlock(mhcode, codec uint64, maxSize, idSize int, bldrFn func(cid.Cid) (Block, error)) {
	mh.Register(mhcode, func() hash.Hash {
		return &hasher{IDSize: idSize}
	})
	specRegistry[mhcode] = blockSpec{
		idSize:  idSize,
		maxSize: maxSize,
		codec:   codec,
		builder: bldrFn,
	}
}

// blockSpec holds constant metadata about particular Block types.
type blockSpec struct {
	idSize  int
	maxSize int
	codec   uint64
	builder func(cid.Cid) (Block, error)
}

func (spec *blockSpec) String() string {
	return fmt.Sprintf("BlockSpec{IDSize: %d, Codec: %d}", spec.idSize, spec.codec)
}

var specRegistry = make(map[uint64]blockSpec)
