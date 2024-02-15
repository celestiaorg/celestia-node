package shwap

import (
	"context"
	"fmt"
	"testing"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/store/file"
)

type TestBlockstore struct {
	t          *testing.T
	lastHeight uint64
	blocks     map[uint64]*file.MemFile
}

func NewTestBlockstore(t *testing.T) *TestBlockstore {
	return &TestBlockstore{
		t:          t,
		lastHeight: 1,
		blocks:     make(map[uint64]*file.MemFile),
	}
}

func (t *TestBlockstore) AddEds(eds *rsmt2d.ExtendedDataSquare) (height uint64) {
	for {
		if _, ok := t.blocks[t.lastHeight]; !ok {
			break
		}
		t.lastHeight++
	}
	t.blocks[t.lastHeight] = &file.MemFile{Eds: eds}
	return t.lastHeight
}

func (t *TestBlockstore) DeleteBlock(ctx context.Context, cid cid.Cid) error {
	//TODO implement me
	panic("not implemented")
}

func (t *TestBlockstore) Has(ctx context.Context, cid cid.Cid) (bool, error) {
	req, err := BlockBuilderFromCID(cid)
	if err != nil {
		return false, fmt.Errorf("while getting height from CID: %w", err)
	}

	_, ok := t.blocks[req.GetHeight()]
	return ok, nil
}

func (t *TestBlockstore) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	req, err := BlockBuilderFromCID(cid)
	if err != nil {
		return nil, fmt.Errorf("while getting height from CID: %w", err)
	}

	f, ok := t.blocks[req.GetHeight()]
	if !ok {
		return nil, ipld.ErrNotFound{Cid: cid}
	}
	return req.BlockFromFile(ctx, f)
}

func (t *TestBlockstore) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	req, err := BlockBuilderFromCID(cid)
	if err != nil {
		return 0, fmt.Errorf("while getting height from CID: %w", err)
	}

	f, ok := t.blocks[req.GetHeight()]
	if !ok {
		return 0, ipld.ErrNotFound{Cid: cid}
	}
	return f.Size(), nil
}

func (t *TestBlockstore) Put(ctx context.Context, block blocks.Block) error {
	panic("not implemented")
}

func (t *TestBlockstore) PutMany(ctx context.Context, blocks []blocks.Block) error {
	panic("not implemented")
}

func (t *TestBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	panic("not implemented")
}

func (t *TestBlockstore) HashOnRead(enabled bool) {
	panic("not implemented")
}
