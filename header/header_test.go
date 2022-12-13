package header

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/core"
)

func TestMakeExtendedHeaderForEmptyBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	_, client := core.StartTestCoreWithApp(t)
	fetcher := core.NewBlockFetcher(client)

	sub, err := fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)
	<-sub

	height := int64(1)
	b, err := fetcher.GetBlock(ctx, &height)
	require.NoError(t, err)

	comm, val, err := fetcher.GetBlockInfo(ctx, &height)
	require.NoError(t, err)

	noop := func(ctx context.Context, root []byte, square *rsmt2d.ExtendedDataSquare) error { return nil }
	headerExt, err := MakeExtendedHeader(ctx, b, comm, val, noop)
	require.NoError(t, err)

	assert.Equal(t, EmptyDAH(), *headerExt.DAH)
}

func TestMismatchedDataHash_ComputedRoot(t *testing.T) {
	header := RandExtendedHeader(t)

	header.DataHash = rand.Bytes(32)

	err := header.ValidateBasic()
	assert.ErrorContains(t, err, "mismatch between data hash")
}
