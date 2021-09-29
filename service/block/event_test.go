package block

import (
	"context"
	"testing"

	"github.com/celestiaorg/celestia-core/testutils"

	"github.com/stretchr/testify/require"
)

func Test_listenForNewBlocks(t *testing.T) {
	mockFetcher := &mockFetcher{
		mockNewBlockCh: make(chan *RawBlock),
	}
	serv := NewBlockService(mockFetcher)

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
	for i := 0; i < num; i++ {
		data, err := testutils.GenerateRandomBlockData(1, 1, 1, 1, 40)
		if err != nil {
			t.Fatal(err)
		}
		m.mockNewBlockCh <- &RawBlock{
			Data: data,
		}
	}
}
