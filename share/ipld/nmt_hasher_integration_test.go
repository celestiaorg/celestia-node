package ipld_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	mrand "math/rand"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/availability/full"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/service"
)

var _ blockstore.Blockstore = (*MockBlockstore)(nil)

// CorruptBlock is a block where the cid doesn't match the data.
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

type MockBlockstore struct {
	ds.Datastore
	attacking bool
}

func (m MockBlockstore) Has(ctx context.Context, cid cid.Cid) (bool, error) {
	return false, nil
}

func (m MockBlockstore) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	key := cid.String()
	if m.attacking {
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
	if m.attacking {
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

func MockNode(t *testing.T, net *availability_test.DagNet) (*availability_test.Node, *MockBlockstore) {
	t.Helper()
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	mockBS := &MockBlockstore{
		Datastore: dstore,
		attacking: false,
	}
	provider := net.NodeWithBlockStore(dstore, mockBS)
	provider.ShareService = service.NewShareService(provider.BlockService, full.TestAvailability(provider.BlockService))
	return provider, mockBS
}

// TestNamespaceHasher_CorruptedData is an integration test that
// verifies that the NamespaceHasher of a recipient of corrupted data will not panic,
// and will throw away the corrupted data.
func TestNamespaceHasher_CorruptedData(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	net := availability_test.NewTestDAGNet(ctx, t)

	requestor := full.Node(net)
	provider, mockBS := MockNode(t, net)
	net.ConnectAll()

	// before the provider starts attacking, we should be able to retrieve successfully
	root := availability_test.RandFillBS(t, 16, provider.BlockService)
	getCtx, cancelGet := context.WithTimeout(ctx, 2*time.Second)
	t.Cleanup(cancelGet)
	err := requestor.SharesAvailable(getCtx, root)
	require.NoError(t, err)

	// clear the storage of the requestor so that it must retrieve again, then start attacking
	requestor.ClearStorage()
	mockBS.attacking = true
	getCtx, cancelGet = context.WithTimeout(ctx, 2*time.Second)
	t.Cleanup(cancelGet)
	err = requestor.SharesAvailable(getCtx, root)
	require.Error(t, err, "availability vailidation failed")
}
