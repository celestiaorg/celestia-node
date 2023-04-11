package availability_test

import (
	"context"
	"crypto/rand"
	"fmt"
	mrand "math/rand"
	"testing"

	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	blocks "github.com/ipfs/go-libipfs/blocks"
)

var _ blockstore.Blockstore = (*FraudulentBlockstore)(nil)

// CorruptBlock is a block where the cid doesn't match the data. It fulfills the blocks.Block
// interface.
type CorruptBlock struct {
	cid  cid.Cid
	data []byte
}

func (b *CorruptBlock) RawData() []byte {
	return b.data
}

func (b *CorruptBlock) Cid() cid.Cid {
	return b.cid
}

func (b *CorruptBlock) String() string {
	return fmt.Sprintf("[Block %s]", b.Cid())
}

func (b *CorruptBlock) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"block": b.Cid().String(),
	}
}

func NewCorruptBlock(data []byte, fakeCID cid.Cid) *CorruptBlock {
	return &CorruptBlock{
		fakeCID,
		data,
	}
}

// FraudulentBlockstore is a mock blockstore.Blockstore that saves both corrupted and original data
// for every block it receives. If FraudulentBlockstore.Attacking is true, it will serve the
// corrupted data on requests.
type FraudulentBlockstore struct {
	ds.Datastore
	Attacking bool
}

func (fb FraudulentBlockstore) Has(context.Context, cid.Cid) (bool, error) {
	return false, nil
}

func (fb FraudulentBlockstore) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	key := cid.String()
	if fb.Attacking {
		key = "corrupt" + key
	}

	data, err := fb.Datastore.Get(ctx, ds.NewKey(key))
	if err != nil {
		return nil, err
	}
	return NewCorruptBlock(data, cid), nil
}

func (fb FraudulentBlockstore) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	key := cid.String()
	if fb.Attacking {
		key = "corrupt" + key
	}

	return fb.Datastore.GetSize(ctx, ds.NewKey(key))
}

func (fb FraudulentBlockstore) Put(ctx context.Context, block blocks.Block) error {
	err := fb.Datastore.Put(ctx, ds.NewKey(block.Cid().String()), block.RawData())
	if err != nil {
		return err
	}

	// create data that doesn't match the CID with arbitrary lengths between 1 and
	// len(block.RawData())*2
	corrupted := make([]byte, 1+mrand.Int()%(len(block.RawData())*2-1)) //nolint:gosec
	_, _ = rand.Read(corrupted)
	return fb.Datastore.Put(ctx, ds.NewKey("corrupt"+block.Cid().String()), corrupted)
}

func (fb FraudulentBlockstore) PutMany(ctx context.Context, blocks []blocks.Block) error {
	for _, b := range blocks {
		err := fb.Put(ctx, b)
		if err != nil {
			return err
		}
	}
	return nil
}

func (fb FraudulentBlockstore) DeleteBlock(context.Context, cid.Cid) error {
	panic("implement me")
}

func (fb FraudulentBlockstore) AllKeysChan(context.Context) (<-chan cid.Cid, error) {
	panic("implement me")
}

func (fb FraudulentBlockstore) HashOnRead(bool) {
	panic("implement me")
}

// MockNode creates a TestNode that uses a FraudulentBlockstore to simulate serving corrupted data.
func MockNode(t *testing.T, net *TestDagNet) (*TestNode, *FraudulentBlockstore) {
	t.Helper()
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	mockBS := &FraudulentBlockstore{
		Datastore: dstore,
		Attacking: false,
	}
	provider := net.NewTestNodeWithBlockstore(dstore, mockBS)
	return provider, mockBS
}
