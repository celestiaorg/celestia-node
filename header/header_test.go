package header

import (
	"context"
	"testing"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/share"
)

func TestMakeExtendedHeaderForEmptyBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	_, client := core.StartTestCoreWithApp(t)
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

	headerExt, err := MakeExtendedHeader(ctx, b, comm, val, store)
	require.NoError(t, err)

	assert.Equal(t, EmptyDAH(), *headerExt.DAH)
}

func TestMismatchedDataHash_ComputedRoot(t *testing.T) {
	header := RandExtendedHeader(t)

	dah := da.NewDataAvailabilityHeader(share.RandEDS(t, 4))
	header.DAH = &dah

	err := header.ValidateBasic()
	assert.ErrorContains(t, err, "mismatch between data hash")
}
