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
	case sampleCodec:
		id, err := SampleIDFromCID(cid)
		if err != nil {
			err = fmt.Errorf("while converting CID to SampleId: %w", err)
			log.Error(err)
			return nil, err
		}

		blk, err := b.getSampleBlock(id)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		return blk, nil
	case axisCodec:
		id, err := AxisIDFromCID(cid)
		if err != nil {
			err = fmt.Errorf("while converting CID to AxisID: %w", err)
			log.Error(err)
			return nil, err
		}

		blk, err := b.getAxisBlock(id)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		return blk, nil
	default:
		return nil, fmt.Errorf("unsupported codec")
	}
}

func (b Blockstore[F]) getSampleBlock(id SampleID) (blocks.Block, error) {
	f, err := b.fs.File(id.Height)
	if err != nil {
		return nil, fmt.Errorf("while getting EDS file from FS: %w", err)
	}

	shr, prf, err := f.ShareWithProof(id.AxisType, int(id.AxisIndex), int(id.ShareIndex))
	if err != nil {
		return nil, fmt.Errorf("while getting share with proof: %w", err)
	}

	s := NewSample(id, shr, prf, f.Size())
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

func (b Blockstore[F]) getAxisBlock(id AxisID) (blocks.Block, error) {
	f, err := b.fs.File(id.Height)
	if err != nil {
		return nil, fmt.Errorf("while getting EDS file from FS: %w", err)
	}

	axisHalf, err := f.AxisHalf(id.AxisType, int(id.AxisIndex))
	if err != nil {
		return nil, fmt.Errorf("while getting axis half: %w", err)
	}

	s := NewAxis(id, axisHalf)
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
	//  allocating Sample's block.Block.
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
	case sampleCodec:
		id, err := SampleIDFromCID(cid)
		if err != nil {
			err = fmt.Errorf("while converting CID to SampleID: %w", err)
			log.Error(err)
			return false, err
		}

		height = id.Height
	case axisCodec:
		id, err := AxisIDFromCID(cid)
		if err != nil {
			err = fmt.Errorf("while converting CID to AxisID: %w", err)
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
