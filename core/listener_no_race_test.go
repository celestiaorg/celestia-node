//go:build !race

package core

import (
	"bytes"
	"context"
	"github.com/celestiaorg/celestia-app/v4/test/util/testnode"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
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
	cfg := DefaultTestConfig().WithChainID(testChainID)
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

		resp, err := cctx.FillBlock(16, accounts[0].Name, flags.BroadcastSync)
		require.NoError(t, err)
		require.NotEmpty(t, resp.TxHash)

		resp, err = waitForTxResponse(cctx, resp.TxHash, time.Second*10)
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

// waitForTxResponse polls for a tx hash and returns a full sdk.TxResponse
func waitForTxResponse(cctx testnode.Context, txHash string, timeout time.Duration) (*sdk.TxResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			fullyPopulatedTxResp, err := authtx.QueryTx(cctx.Context, txHash)
			if err != nil {
				continue
			}
			return fullyPopulatedTxResp, nil

		}
	}
}
