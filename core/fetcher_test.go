package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/bytes"
)

func TestBlockFetcher_GetBlock_and_SubscribeNewBlockEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	_, client := StartTestClient(ctx, t)
	fetcher := NewBlockFetcher(client)

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
}

// TestBlockFetcherHeaderValues tests that both the Commit and ValidatorSet
// endpoints are working as intended.
func TestBlockFetcherHeaderValues(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	_, client := StartTestClient(ctx, t)
	fetcher := NewBlockFetcher(client)

	// generate some blocks
	newBlockChan, err := fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)
	// read once from channel to generate next block
	<-newBlockChan
	// get Commit from current height
	commit, err := fetcher.Commit(ctx, nil)
	require.NoError(t, err)
	// get ValidatorSet from current height
	valSet, err := fetcher.ValidatorSet(ctx, nil)
	require.NoError(t, err)
	// get next block
	nextBlock := <-newBlockChan
	// compare LastCommit from next block to Commit from first block height
	assert.Equal(t, nextBlock.LastCommit.Hash(), commit.Hash())
	assert.Equal(t, nextBlock.LastCommit.Height, commit.Height)
	assert.Equal(t, nextBlock.LastCommit.Signatures, commit.Signatures)
	// compare ValidatorSet hash to the ValidatorsHash from first block height
	hexBytes := bytes.HexBytes{}
	err = hexBytes.Unmarshal(valSet.Hash())
	require.NoError(t, err)
	assert.Equal(t, nextBlock.ValidatorsHash, hexBytes)

	require.NoError(t, fetcher.UnsubscribeNewBlockEvent(ctx))
}
