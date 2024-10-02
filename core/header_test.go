package core

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/celestia-app/v2/pkg/appconsts"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
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

	eds, err := extendBlock(b.Data, b.Header.Version.App)
	require.NoError(t, err)

	headerExt, err := header.MakeExtendedHeader(&b.Header, comm, val, eds)
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
	header.RawHeader.Version.App = appconsts.LatestVersion + 1

	err := header.Validate()
	assert.Contains(t, err.Error(), fmt.Sprintf("has version %d, this node supports up to version %d. Please "+
		"upgrade to support new version", header.RawHeader.Version.App, appconsts.LatestVersion))
}
