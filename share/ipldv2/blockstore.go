package ipldv2

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/blockstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-node/share/eds"
)

// fileStore is a mocking friendly local interface over eds.FileStore
// TODO(@Wondertan): Consider making an actual interface of eds pkg
type fileStore[F eds.File] interface {
	File(height uint64) (F, error)
}

type Blockstore[F eds.File] struct {
	fs fileStore[F]
}

func NewBlockstore[F eds.File](fs fileStore[F]) blockstore.Blockstore {
	return &Blockstore[F]{fs}
}

func (b Blockstore[F]) Get(_ context.Context, cid cid.Cid) (blocks.Block, error) {
	switch cid.Type() {
	case shareSamplingCodec:
		id, err := ShareSampleIDFromCID(cid)
		if err != nil {
			err = fmt.Errorf("while converting CID to ShareSampleId: %w", err)
			log.Error(err)
			return nil, err
		}

		blk, err := b.getShareSampleBlock(id)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		return blk, nil
	case axisSamplingCodec:
		id, err := AxisSampleIDFromCID(cid)
		if err != nil {
			err = fmt.Errorf("while converting CID to AxisSampleID: %w", err)
			log.Error(err)
			return nil, err
		}

		blk, err := b.getAxisSampleBlock(id)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		return blk, nil
	default:
		return nil, fmt.Errorf("unsupported codec")
	}
}

func (b Blockstore[F]) getShareSampleBlock(id ShareSampleID) (blocks.Block, error) {
	f, err := b.fs.File(id.Height)
	if err != nil {
		return nil, fmt.Errorf("while getting EDS file from FS: %w", err)
	}

	shr, prf, err := f.ShareWithProof(id.Index, id.Axis)
	if err != nil {
		return nil, fmt.Errorf("while getting share with proof: %w", err)
	}

	s := NewShareSample(id, shr, prf, f.Size())
	blk, err := s.IPLDBlock()
	if err != nil {
		return nil, fmt.Errorf("while coverting to IPLD block: %w", err)
	}

	err = f.Close()
	if err != nil {
		return nil, fmt.Errorf("while closing EDS file: %w", err)
	}

	return blk, nil
}

func (b Blockstore[F]) getAxisSampleBlock(id AxisSampleID) (blocks.Block, error) {
	f, err := b.fs.File(id.Height)
	if err != nil {
		return nil, fmt.Errorf("while getting EDS file from FS: %w", err)
	}

	axisHalf, err := f.AxisHalf(id.Index, id.Axis)
	if err != nil {
		return nil, fmt.Errorf("while getting axis half: %w", err)
	}

	s := NewAxisSample(id, axisHalf)
	blk, err := s.IPLDBlock()
	if err != nil {
		return nil, fmt.Errorf("while coverting to IPLD block: %w", err)
	}

	err = f.Close()
	if err != nil {
		return nil, fmt.Errorf("while closing EDS file: %w", err)
	}

	return blk, nil
}

func (b Blockstore[F]) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	// TODO(@Wondertan): There must be a way to derive size without reading, proving, serializing and
	//  allocating ShareSample's block.Block.
	// NOTE:Bitswap uses GetSize also to determine if we have content stored or not
	// so simply returning constant size is not an option
	blk, err := b.Get(ctx, cid)
	if err != nil {
		return 0, err
	}

	return len(blk.RawData()), nil
}

func (b Blockstore[F]) Has(_ context.Context, cid cid.Cid) (bool, error) {
	var height uint64
	switch cid.Type() {
	case shareSamplingCodec:
		id, err := ShareSampleIDFromCID(cid)
		if err != nil {
			err = fmt.Errorf("while converting CID to ShareSampleID: %w", err)
			log.Error(err)
			return false, err
		}

		height = id.Height
	case axisSamplingCodec:
		id, err := AxisSampleIDFromCID(cid)
		if err != nil {
			err = fmt.Errorf("while converting CID to AxisSampleID: %w", err)
			log.Error(err)
			return false, err
		}

		height = id.Height
	default:
		return false, fmt.Errorf("unsupported codec")
	}

	f, err := b.fs.File(height)
	if err != nil {
		err = fmt.Errorf("while getting EDS file from FS: %w", err)
		log.Error(err)
		return false, err
	}

	err = f.Close()
	if err != nil {
		err = fmt.Errorf("while closing EDS file: %w", err)
		log.Error(err)
		return false, err
	}
	// existence of the file confirms existence of the share
	return true, nil
}

func (b Blockstore[F]) AllKeysChan(context.Context) (<-chan cid.Cid, error) {
	return nil, fmt.Errorf("AllKeysChan is unsupported")
}

func (b Blockstore[F]) DeleteBlock(context.Context, cid.Cid) error {
	return fmt.Errorf("writes are not supported")
}

func (b Blockstore[F]) Put(context.Context, blocks.Block) error {
	return fmt.Errorf("writes are not supported")
}

func (b Blockstore[F]) PutMany(context.Context, []blocks.Block) error {
	return fmt.Errorf("writes are not supported")
}

func (b Blockstore[F]) HashOnRead(bool) {}
