package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockFetcher_GetBlock_and_SubscribeNewBlockEvent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	client := StartTestNode(t).Client
	fetcher := NewBlockFetcher(client)

	// generate some blocks
	newBlockChan, err := fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)

	for i := 1; i < 3; i++ {
		select {
		case newBlockFromChan := <-newBlockChan:
			h := newBlockFromChan.Header.Height
			block, err := fetcher.GetSignedBlock(ctx, &h)
			require.NoError(t, err)
			assert.Equal(t, newBlockFromChan.Data, block.Data)
			assert.Equal(t, newBlockFromChan.Header, block.Header)
			assert.Equal(t, newBlockFromChan.Commit, block.Commit)
			assert.Equal(t, newBlockFromChan.ValidatorSet, block.ValidatorSet)
			require.GreaterOrEqual(t, newBlockFromChan.Header.Height, int64(i))
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		}
	}
	require.NoError(t, fetcher.UnsubscribeNewBlockEvent(ctx))
}
