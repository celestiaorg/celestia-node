package block

import (
	"context"
	"testing"

	"github.com/celestiaorg/celestia-core/testutils"
	md "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/service/header"
)

func Test_listenForNewBlocks(t *testing.T) {
	mockFetcher := &mockFetcher{
		mockNewBlockCh: make(chan *RawBlock),
	}
	serv := NewBlockService(mockFetcher, md.Mock()) // TODO @renaynay: add mock dag service

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err := serv.Start(ctx)
	require.NoError(t, err)

	mockFetcher.generateBlocks(t, 3)

	err = serv.Stop(ctx)
	require.NoError(t, err)
}

// mockFetcher mocks away the `Fetcher` interface.
type mockFetcher struct {
	mockNewBlockCh chan *RawBlock
}

func (m *mockFetcher) GetBlock(ctx context.Context, height *int64) (*RawBlock, error) {
	return nil, nil
}

func (m *mockFetcher) SubscribeNewBlockEvent(ctx context.Context) (<-chan *RawBlock, error) {
	return m.mockNewBlockCh, nil
}

func (m *mockFetcher) UnsubscribeNewBlockEvent(ctx context.Context) error {
	close(m.mockNewBlockCh)
	return nil
}

func (m *mockFetcher) generateBlocks(t *testing.T, num int) {
	t.Helper()

	for i := 0; i < num; i++ {
		rawBlock, _ := generateRawAndExtendedBlock(t)
		m.mockNewBlockCh <- rawBlock
	}
}

func generateRawAndExtendedBlock(t *testing.T) (*RawBlock, *Block) {
	t.Helper()

	data, err := testutils.GenerateRandomBlockData(1, 1, 1, 1, 40)
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
