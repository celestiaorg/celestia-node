package core

import (
	"context"
	"fmt"
	"testing"

	"github.com/cometbft/cometbft/libs/rand"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v7/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v7/pkg/da"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
)

func TestMakeExtendedHeaderForEmptyBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	network := NewNetwork(t, DefaultTestConfig())
	require.NoError(t, network.Start())
	t.Cleanup(func() {
		require.NoError(t, network.Stop())
	})

	fetcher, err := NewBlockFetcher(network.GRPCClient)
	require.NoError(t, err)
	sub, err := fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)
	dataBlock := <-sub

	height := dataBlock.Header.Height
	b, err := fetcher.GetBlock(ctx, height)
	require.NoError(t, err)

	comm, val, err := fetcher.GetBlockInfo(ctx, height)
	require.NoError(t, err)

	eds, err := da.ConstructEDS(b.Data.Txs.ToSliceOfBytes(), b.Header.Version.App, -1)
	require.NoError(t, err)

	headerExt, err := header.MakeExtendedHeader(b.Header, comm, val, eds)
	require.NoError(t, err)

	assert.Equal(t, share.EmptyEDSRoots(), headerExt.DAH)
}

func TestMismatchedDataHash_ComputedRoot(t *testing.T) {
	header := headertest.RandExtendedHeader(t)
	header.DataHash = rand.Bytes(32)

	err := header.Validate()
	assert.Contains(t, err.Error(), "mismatch between data hash commitment from"+
		" core header and computed data root")
}

func TestBadAppVersion(t *testing.T) {
	header := headertest.RandExtendedHeader(t)
	header.Version.App = appconsts.Version + 1

	err := header.Validate()
	assert.Contains(t, err.Error(), fmt.Sprintf("has version %d, this node supports up to version %d. Please "+
		"upgrade to support new version", header.Version.App, appconsts.Version))
}
