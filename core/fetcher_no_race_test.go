//go:build !race

package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBlockFetcherHeaderValues tests that both the Commit and ValidatorSet
// endpoints are working as intended.
func TestBlockFetcherHeaderValues(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	network := NewNetwork(t, DefaultTestConfig())
	require.NoError(t, network.Start())
	t.Cleanup(func() {
		require.NoError(t, network.Stop())
	})

	fetcher, err := NewBlockFetcher(network.GRPCClient)
	require.NoError(t, err)
	newBlockChan, err := fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)
	// read once from channel to generate next block
	var h int64
	select {
	case evt := <-newBlockChan:
		h = evt.Header.Height
	case <-ctx.Done():
		require.NoError(t, ctx.Err())
	}
	// get Commit from current height
	commit, err := fetcher.Commit(ctx, h)
	require.NoError(t, err)
	// get ValidatorSet from current height
	valSet, err := fetcher.ValidatorSet(ctx, h)
	require.NoError(t, err)
	// get next block
	var nextBlock SignedBlock
	select {
	case nextBlock = <-newBlockChan:
	case <-ctx.Done():
		require.NoError(t, ctx.Err())
	}
	// compare LastCommit from next block to Commit from first block height
	assert.Equal(t, nextBlock.Header.LastCommitHash, commit.Hash())
	assert.Equal(t, nextBlock.Header.Height, commit.Height+1)
	// compare ValidatorSet hash to the ValidatorsHash from first block height
	hexBytes := valSet.Hash()
	assert.Equal(t, nextBlock.ValidatorSet.Hash(), hexBytes)
}
