//go:build !race

package core

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/store"
)

// TestListenerWithNonEmptyBlocks ensures that non-empty blocks are actually
// stored to eds.Store.
func TestListenerWithNonEmptyBlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	// create mocknet with two pubsub endpoints
	ps0, _ := createMocknetWithTwoPubsubEndpoints(ctx, t)

	// create one block to store as Head in local store and then unsubscribe from block events
	cfg := DefaultTestConfig()
	fetcher, cctx := createCoreFetcher(t, cfg)
	eds := createEdsPubSub(ctx, t)

	store, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	// create Listener and start listening
	cl := createListener(ctx, t, fetcher, ps0, eds, store, testChainID)
	err = cl.Start(ctx)
	require.NoError(t, err)

	// listen for eds hashes broadcasted through eds-sub and ensure store has
	// already stored them
	sub, err := eds.Subscribe()
	require.NoError(t, err)
	t.Cleanup(sub.Cancel)

	empty := share.EmptyEDSRoots()
	// TODO extract 16
	for i := 0; i < 16; i++ {
		accounts := cfg.Genesis.Accounts()
		require.Greater(t, len(accounts), 0)
		_, err := cctx.FillBlock(16, accounts[0].Name, flags.BroadcastBlock)
		require.NoError(t, err)
		msg, err := sub.Next(ctx)
		require.NoError(t, err)

		if bytes.Equal(empty.Hash(), msg.DataHash) {
			continue
		}

		has, err := store.HasByHash(ctx, msg.DataHash)
		require.NoError(t, err)
		require.True(t, has)

		has, err = store.HasByHeight(ctx, msg.Height)
		require.NoError(t, err)
		require.True(t, has)
	}

	err = cl.Stop(ctx)
	require.NoError(t, err)
	require.Nil(t, cl.cancel)
}
