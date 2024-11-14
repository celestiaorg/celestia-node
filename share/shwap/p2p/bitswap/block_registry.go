package bitswap

import (
	"fmt"
	"hash"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

// EmptyBlock constructs an empty Block with type in the given CID.
func EmptyBlock(cid cid.Cid) (Block, error) {
	spec, err := getSpec(cid)
	if err != nil {
		return nil, err
	}

	blk, err := spec.builder(cid)
	if err != nil {
		return nil, fmt.Errorf("failed to build a Block for %s: %w", spec.String(), err)
	}

	return blk, nil
}

// maxBlockSize returns the maximum size of the Block type in the given CID.
func maxBlockSize(cid cid.Cid) (int, error) {
	spec, err := getSpec(cid)
	if err != nil {
		return 0, err
	}

	return spec.maxSize, nil
}

// registerBlock registers the new Block type and multihash for it.
func registerBlock(mhcode, codec uint64, maxSize, idSize int, bldrFn func(cid.Cid) (Block, error)) {
	mh.Register(mhcode, func() hash.Hash {
		return &hasher{IDSize: idSize}
	})
	specRegistry[codec] = blockSpec{
		idSize:  idSize,
		maxSize: maxSize,
		mhCode:  mhcode,
		builder: bldrFn,
	}
}

// getSpec returns the blockSpec for the given CID.
func getSpec(cid cid.Cid) (blockSpec, error) {
	spec, ok := specRegistry[cid.Type()]
	if !ok {
		return blockSpec{}, fmt.Errorf("unsupported codec %d", cid.Type())
	}

	return spec, nil
}

// blockSpec holds constant metadata about particular Block types.
type blockSpec struct {
	idSize  int
	maxSize int
	mhCode  uint64
	builder func(cid.Cid) (Block, error)
}

func (spec *blockSpec) String() string {
	return fmt.Sprintf("BlockSpec{IDSize: %d, MHCode: %d}", spec.idSize, spec.mhCode)
}

var specRegistry = make(map[uint64]blockSpec)
