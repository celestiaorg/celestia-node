package ipldv2

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/boxo/blockstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
)

// edsFile is a mocking friendly local interface over eds.File.
// TODO(@Wondertan): Consider making an actual interface of eds pkg
type edsFile interface {
	io.Closer
	Header() *eds.Header
	ShareWithProof(idx int, axis rsmt2d.Axis) (share.Share, nmt.Proof, error)
}

// fileStore is a mocking friendly local interface over eds.FileStore
// TODO(@Wondertan): Consider making an actual interface of eds pkg
type fileStore[F edsFile] interface {
	File(share.DataHash) (F, error)
}

type Blockstore[F edsFile] struct {
	fs fileStore[F]
}

func NewBlockstore[F edsFile](fs fileStore[F]) blockstore.Blockstore {
	return &Blockstore[F]{fs}
}

func (b Blockstore[F]) Get(_ context.Context, cid cid.Cid) (blocks.Block, error) {
	id, err := SampleIDFromCID(cid)
	if err != nil {
		err = fmt.Errorf("while converting CID to SampleID: %w", err)
		log.Error(err)
		return nil, err
	}

	f, err := b.fs.File(id.DataHash)
	if err != nil {
		err = fmt.Errorf("while getting EDS file from FS: %w", err)
		log.Error(err)
		return nil, err
	}

	shr, prf, err := f.ShareWithProof(id.Index, id.Axis)
	if err != nil {
		err = fmt.Errorf("while getting share with proof: %w", err)
		log.Error(err)
		return nil, err
	}

	s := NewSample(id, shr, prf, f.Header().SquareSize())
	blk, err := s.IPLDBlock()
	if err != nil {
		err = fmt.Errorf("while getting share with proof: %w", err)
		log.Error(err)
		return nil, err
	}

	err = f.Close()
	if err != nil {
		err = fmt.Errorf("while closing EDS file: %w", err)
		log.Error(err)
		return nil, err
	}

	return blk, nil
}

func (b Blockstore[F]) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	// TODO(@Wondertan): There must be a way to derive size without reading, proving, serializing and
	// allocating Sample's block.Block.
	blk, err := b.Get(ctx, cid)
	if err != nil {
		return 0, err
	}

	return len(blk.RawData()), nil
}

func (b Blockstore[F]) Has(_ context.Context, cid cid.Cid) (bool, error) {
	id, err := SampleIDFromCID(cid)
	if err != nil {
		err = fmt.Errorf("while converting CID to SampleID: %w", err)
		log.Error(err)
		return false, err
	}

	f, err := b.fs.File(id.DataHash)
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
