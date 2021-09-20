package block

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_listenForNewBlocks(t *testing.T) {
	mockFetcher := &mockFetcher{
		mockNewBlockCh: make(chan *Raw),
	}
	serv := NewBlockService(mockFetcher)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err := serv.Start(ctx)
	require.NoError(t, err)
	require.NotNil(t, serv.cancelListen)

	mockFetcher.generateBlocks(3)

	err = serv.Stop(ctx)
	require.NoError(t, err)
	require.Nil(t, serv.cancelListen)
}

// mockFetcher
type mockFetcher struct {
	mockNewBlockCh chan *Raw
}

func (m *mockFetcher) GetBlock(ctx context.Context, height *int64) (*Raw, error) {
	return nil, nil
}

func (m *mockFetcher) SubscribeNewBlockEvent(ctx context.Context) (<-chan *Raw, error) {
	return m.mockNewBlockCh, nil
}

func (m *mockFetcher) UnsubscribeNewBlockEvent(ctx context.Context) error {
	close(m.mockNewBlockCh)
	return nil
}

func (m *mockFetcher) generateBlocks(num int) {
	for i := 0; i < num; i++ {
		m.mockNewBlockCh <- &Raw{}
	}
}
