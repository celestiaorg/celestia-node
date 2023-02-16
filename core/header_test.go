package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/headertest"
)

func TestMakeExtendedHeaderForEmptyBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client := StartTestNode(t).Client
	fetcher := NewBlockFetcher(client)

	sub, err := fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)
	<-sub

	height := int64(1)
	b, err := fetcher.GetBlock(ctx, &height)
	require.NoError(t, err)

	comm, val, err := fetcher.GetBlockInfo(ctx, &height)
	require.NoError(t, err)

	eds, err := extendBlock(b.Data)
	require.NoError(t, err)

	headerExt, err := header.MakeExtendedHeader(ctx, b, comm, val, eds)
	require.NoError(t, err)

	assert.Equal(t, header.EmptyDAH(), *headerExt.DAH)
}

func TestMismatchedDataHash_ComputedRoot(t *testing.T) {
	header := headertest.RandExtendedHeader(t)

	header.DataHash = rand.Bytes(32)

	err := header.Validate()
	assert.ErrorContains(t, err, "mismatch between data hash")
}
