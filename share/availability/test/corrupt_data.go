package availability_test

import (
	"context"
	"fmt"
	mrand "math/rand"
	"testing"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
)

var _ blockstore.Blockstore = (*MockBlockstore)(nil)

// CorruptBlock is a block where the cid doesn't match the data.
// It fulfills the blocks.Block interface.
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

// MockBlockstore is a mock blockstore.Blockstore that saves both corrupted and original data for every block it
// receives. If MockBlockstore.Attacking is true, it will serve the corrupted data on requests.
type MockBlockstore struct {
	ds.Datastore
	Attacking bool
}

func (m MockBlockstore) Has(ctx context.Context, cid cid.Cid) (bool, error) {
	return false, nil
}

func (m MockBlockstore) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	key := cid.String()
	if m.Attacking {
		key = "corrupt" + key
	}

	data, err := m.Datastore.Get(ctx, ds.NewKey(key))
	if err != nil {
		return nil, err
	}
	return NewCorruptBlock(data, cid), nil
}

func (m MockBlockstore) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	key := cid.String()
	if m.Attacking {
		key = "corrupt" + key
	}

	return m.Datastore.GetSize(ctx, ds.NewKey(key))
}

func (m MockBlockstore) Put(ctx context.Context, block blocks.Block) error {
	err := m.Datastore.Put(ctx, ds.NewKey(block.Cid().String()), block.RawData())
	if err != nil {
		return err
	}

	// create data that doesn't match the CID with arbitrary lengths between 0 and len(block.RawData())*2
	corrupted := make([]byte, mrand.Int()%(len(block.RawData())*2))
	mrand.Read(corrupted)
	return m.Datastore.Put(ctx, ds.NewKey("corrupt"+block.Cid().String()), corrupted)
}

func (m MockBlockstore) PutMany(ctx context.Context, blocks []blocks.Block) error {
	for _, b := range blocks {
		err := m.Put(ctx, b)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m MockBlockstore) DeleteBlock(ctx context.Context, cid cid.Cid) error {
	// TODO implement me
	panic("implement me")
}

func (m MockBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	// TODO implement me
	panic("implement me")
}

func (m MockBlockstore) HashOnRead(enabled bool) {
	// TODO implement me
	panic("implement me")
}

// MockNode creates a TestNode that uses a MockBlockstore to simulate serving corrupted data.
func MockNode(t *testing.T, net *TestDagNet) (*TestNode, *MockBlockstore) {
	t.Helper()
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	mockBS := &MockBlockstore{
		Datastore: dstore,
		Attacking: false,
	}
	provider := net.NewTestNodeWithBlockstore(dstore, mockBS)
	return provider, mockBS
}
