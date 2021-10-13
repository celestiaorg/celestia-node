package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockFetcher_GetBlock_and_SubscribeNewBlockEvent(t *testing.T) {
	client := MockEmbeddedClient()
	fetcher := NewBlockFetcher(client)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// generate some blocks
	newBlockChan, err := fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)

	for i := 1; i < 3; i++ {
		newBlockFromChan := <-newBlockChan

		block, err := fetcher.GetBlock(ctx, nil)
		require.NoError(t, err)

		assert.Equal(t, newBlockFromChan, block)
	}

	require.NoError(t, fetcher.UnsubscribeNewBlockEvent(ctx))
	require.NoError(t, client.Stop())
}
