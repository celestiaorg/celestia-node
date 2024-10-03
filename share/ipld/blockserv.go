package ipld

import (
	"context"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	ipld "github.com/ipfs/go-ipld-format"
)

// NewBlockservice constructs Blockservice for fetching NMTrees.
func NewBlockservice(_ blockstore.Blockstore, exchange exchange.Interface) blockservice.BlockService {
	return blockservice.New(bs{}, exchange, blockservice.WithAllowlist(defaultAllowlist))
}

// NewMemBlockservice constructs Blockservice for fetching NMTrees with in-memory blockstore.
func NewMemBlockservice() blockservice.BlockService {
	bstore := blockstore.NewBlockstore(sync.MutexWrap(datastore.NewMapDatastore()))
	return NewBlockservice(bstore, nil)
}

// defaultAllowlist keeps default list of hashes allowed in the network.
var defaultAllowlist allowlist

type allowlist struct{}

func (a allowlist) IsAllowed(code uint64) bool {
	// we allow all codes except home-baked sha256NamespaceFlagged
	return code == sha256NamespaceFlagged
}

type bs struct{}

func (b bs) DeleteBlock(ctx context.Context, cid cid.Cid) error {
	return nil
}

func (b bs) Has(ctx context.Context, cid cid.Cid) (bool, error) {
	return false, nil
}

func (b bs) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	return nil, ipld.ErrNotFound{}
}

func (b bs) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	return 0, ipld.ErrNotFound{}
}

func (b bs) Put(ctx context.Context, block blocks.Block) error {
	return nil
}

func (b bs) PutMany(ctx context.Context, blocks []blocks.Block) error {
	return nil
}

func (b bs) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return nil, ipld.ErrNotFound{}
}

func (b bs) HashOnRead(enabled bool) {
}
