package headertest

import (
	"context"
	"testing"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header"
)

func TestMakeExtendedHeaderForEmptyBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client := core.StartTestNode(t).Client
	fetcher := core.NewBlockFetcher(client)

	store := mdutils.Bserv()

	sub, err := fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)
	<-sub

	height := int64(1)
	b, err := fetcher.GetBlock(ctx, &height)
	require.NoError(t, err)

	comm, val, err := fetcher.GetBlockInfo(ctx, &height)
	require.NoError(t, err)

	headerExt, err := header.MakeExtendedHeader(ctx, b, comm, val, store)
	require.NoError(t, err)

	assert.Equal(t, header.EmptyDAH(), *headerExt.DAH)
}

func TestMismatchedDataHash_ComputedRoot(t *testing.T) {
	header := RandExtendedHeader(t)

	header.DataHash = rand.Bytes(32)

	err := header.Validate()
	assert.ErrorContains(t, err, "mismatch between data hash")
}
