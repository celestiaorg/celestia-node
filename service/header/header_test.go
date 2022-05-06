package header

import (
	"context"
	"testing"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extheader "github.com/celestiaorg/celestia-node/service/header/extheader"
)

func TestMakeExtendedHeaderForEmptyBlock(t *testing.T) {
	fetcher := createCoreFetcher(t)
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

	headerExt, err := extheader.MakeExtendedHeader(ctx, b, comm, val, store)
	require.NoError(t, err)

	assert.Equal(t, extheader.EmptyDAH(), *headerExt.DAH)
}
