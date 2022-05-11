package header

import (
	"context"
	"testing"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"
)

func TestMakeExtendedHeaderForEmptyBlock(t *testing.T) {
	_, client := core.StartTestClient(t)
	fetcher := core.NewBlockFetcher(client)

	store := mdutils.Mock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub, err := fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)
	<-sub

	height := int64(1)
	b, err := fetcher.GetBlock(ctx, &height)
	require.NoError(t, err)

	comm, val, err := fetcher.GetBlockInfo(ctx, &height)
	require.NoError(t, err)

	headerExt, err := MakeExtendedHeader(ctx, b, comm, val, store)
	require.NoError(t, err)

	assert.Equal(t, EmptyDAH(), *headerExt.DAH)
}
