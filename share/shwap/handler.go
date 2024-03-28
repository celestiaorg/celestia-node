package shwap

import (
	"context"
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-node/share/store/file"
)

// BlockBuilder is an interface for building response blocks from request and file.
type BlockBuilder interface {
	// TODO(@walldiss): don't like this name, but it collides with field name in RowID
	GetHeight() uint64
	BlockFromFile(ctx context.Context, file file.EdsFile) (blocks.Block, error)
}

// BlockBuilderFromCID returns a BlockBuilder from a CID. it acts as multiplexer for
// different block types.
func BlockBuilderFromCID(cid cid.Cid) (BlockBuilder, error) {
	switch cid.Type() {
	case sampleCodec:
		h, err := SampleIDFromCID(cid)
		if err != nil {
			return nil, fmt.Errorf("while converting CID to SampleID: %w", err)
		}

		return h, nil
	case rowCodec:
		var err error
		rid, err := RowIDFromCID(cid)
		if err != nil {
			return nil, fmt.Errorf("while converting CID to RowID: %w", err)
		}
		return rid, nil
	case dataCodec:
		did, err := DataIDFromCID(cid)
		if err != nil {
			return nil, fmt.Errorf("while converting CID to DataID: %w", err)
		}
		return did, nil
	default:
		return nil, fmt.Errorf("unsupported codec")
	}
}
