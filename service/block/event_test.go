package block

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	md "github.com/ipfs/go-merkledag/test"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	core "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-node/service/header"
)

// TestEventLoop tests that the Service event loop spawned by calling
// `Start` on the Service properly listens for new blocks from its Fetcher
// and handles them accordingly.
func TestEventLoop(t *testing.T) {
	mockFetcher := &mockFetcher{
		mockNewBlockCh: make(chan *RawBlock),
	}
	serv := NewBlockService(mockFetcher, md.Mock(), new(mockBroadcaster))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err := serv.Start(ctx)
	require.NoError(t, err)

	numBlocks := 3
	expectedBlocks := mockFetcher.generateBlocks(t, numBlocks)

	// as we calculate extended square twice, this may take a lot of time, causing the main thread expect the result
	// earlier than its ready.
	time.Sleep(100 * time.Millisecond)
	for i := 0; i < numBlocks; i++ {
		block, err := serv.GetBlockData(ctx, expectedBlocks[i].Header().DAH)
		require.NoError(t, err)
		assert.Equal(t, expectedBlocks[i].data.Width(), block.Width())
		assert.Equal(t, expectedBlocks[i].data.RowRoots(), block.RowRoots())
		assert.Equal(t, expectedBlocks[i].data.ColRoots(), block.ColRoots())
	}

	err = serv.Stop(ctx)
	require.NoError(t, err)
}

func TestExtendedHeaderBroadcast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	suite := header.NewTestSuite(t, 3)

	mockFetcher := &mockFetcher{
		suite:          suite,
		commits:        make(map[int64]*core.Commit),
		valSets:        make(map[int64]*core.ValidatorSet),
		mockNewBlockCh: make(chan *RawBlock),
	}

	// create mock network w/ new pubsub
	net, err := mocknet.FullMeshConnected(context.Background(), 2)
	require.NoError(t, err)

	pub, err := pubsub.NewGossipSub(context.Background(), net.Hosts()[1],
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign))
	require.NoError(t, err)

	store1, err := header.NewStoreWithHead(datastore.NewMapDatastore(), suite.Head())
	require.NoError(t, err)

	// also create subscription to topic to listen on the other side
	headerServ := header.NewHeaderService(header.NewLocalExchange(store1), store1, pub, suite.Head().Hash())
	require.NoError(t, err)
	err = headerServ.Start(ctx)
	require.NoError(t, err)

	sub, err := headerServ.Subscribe()
	require.NoError(t, err)

	serv := NewBlockService(mockFetcher, md.Mock(), headerServ)

	err = serv.Start(ctx)
	require.NoError(t, err)

	numBlocks := 3
	validRawBlocks := mockFetcher.generateBlocksWithValidHeaders(t, numBlocks)

	i := 0
	for i < numBlocks {
		got, err := sub.NextHeader(context.Background())
		require.NoError(t, err)
		assert.Equal(t, validRawBlocks[i].Header.Height, got.Height)
		assert.Equal(t, validRawBlocks[i].Header.DataHash.Bytes(), got.DAH.Hash())
		i++
	}
}

// mockFetcher mocks away the `Fetcher` interface.
type mockFetcher struct {
	suite          *header.TestSuite
	valSets        map[int64]*core.ValidatorSet
	commits        map[int64]*core.Commit
	mockNewBlockCh chan *RawBlock
}

func (m *mockFetcher) GetBlock(ctx context.Context, height *int64) (*RawBlock, error) {
	return nil, nil
}

func (m *mockFetcher) Commit(ctx context.Context, height *int64) (*core.Commit, error) {
	return m.commits[*height], nil
}

func (m *mockFetcher) ValidatorSet(ctx context.Context, height *int64) (*core.ValidatorSet, error) {
	return m.valSets[*height], nil
}

func (m *mockFetcher) SubscribeNewBlockEvent(ctx context.Context) (<-chan *RawBlock, error) {
	return m.mockNewBlockCh, nil
}

func (m *mockFetcher) UnsubscribeNewBlockEvent(ctx context.Context) error {
	close(m.mockNewBlockCh)
	return nil
}

func (m *mockFetcher) generateBlocksWithValidHeaders(t *testing.T, num int) []*RawBlock {
	rawBlocks := make([]*RawBlock, num)

	prevEH := m.suite.Head()

	for i := range rawBlocks {
		eh := m.suite.GenExtendedHeader()
		b := &RawBlock{
			Header:     eh.RawHeader,
			LastCommit: prevEH.Commit,
		}
		require.NoError(t, b.ValidateBasic())
		require.NoError(t, b.Header.ValidateBasic())

		rawBlocks[i] = b
		// store commit and valset at height
		m.commits[b.Height] = eh.Commit
		m.valSets[b.Height] = eh.ValidatorSet

		m.mockNewBlockCh <- b
		prevEH = eh
	}
	return rawBlocks
}

// generateBlocks generates new raw blocks and sends them to the mock fetcher,
// returning the extended blocks generated from the process to compare against.
func (m *mockFetcher) generateBlocks(t *testing.T, num int) []Block {
	t.Helper()

	extendedBlocks := make([]Block, num)

	for i := 0; i < num; i++ {
		rawBlock, block := generateRawAndExtendedBlock(t)

		extendedBlocks[i] = *block
		m.mockNewBlockCh <- rawBlock
	}

	return extendedBlocks
}

func generateRawAndExtendedBlock(t *testing.T) (*RawBlock, *Block) {
	t.Helper()

	data, err := GenerateRandomBlockData(1, 1, 1, 1, 40)
	if err != nil {
		t.Fatal(err)
	}
	rawBlock := &RawBlock{
		Data: data,
	}
	// extend the data to get the data hash
	extendedData, err := extendBlockData(rawBlock)
	if err != nil {
		t.Fatal(err)
	}
	dah, err := header.DataAvailabilityHeaderFromExtendedData(extendedData)
	if err != nil {
		t.Fatal(err)
	}
	rawBlock.Header = header.RawHeader{
		DataHash: dah.Hash(),
	}
	return rawBlock, &Block{
		header: &header.ExtendedHeader{
			DAH: &dah,
		},
		data: extendedData,
	}
}

type mockBroadcaster struct {
	topic *pubsub.Topic
}

func (mb *mockBroadcaster) Broadcast(ctx context.Context, header *header.ExtendedHeader) error {
	if mb.topic != nil {
		bin, err := header.MarshalBinary()
		if err != nil {
			return err
		}
		if err := mb.topic.Publish(ctx, bin); err != nil {
			return err
		}
	}
	return nil
}
