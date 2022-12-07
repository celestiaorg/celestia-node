package store

import (
	"context"
	"testing"
	"time"

	"github.com/celestiaorg/celestia-node/pkg/header/local"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
)

func TestInitStore_NoReinit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	suite := header.NewTestSuite(t, 3)
	head := suite.Head()
	exchange := local.NewExchange(NewTestStore(ctx, t, head))

	ds := sync.MutexWrap(datastore.NewMapDatastore())
	store, err := NewStore[*header.ExtendedHeader](ds)
	require.NoError(t, err)

	err = Init[*header.ExtendedHeader](ctx, store, exchange, head.Hash())
	assert.NoError(t, err)

	err = store.Start(ctx)
	require.NoError(t, err)

	_, err = store.Append(ctx, suite.GenExtendedHeaders(10)...)
	require.NoError(t, err)

	err = store.Stop(ctx)
	require.NoError(t, err)

	reopenedStore, err := NewStore[*header.ExtendedHeader](ds)
	require.NoError(t, err)

	err = Init[*header.ExtendedHeader](ctx, reopenedStore, exchange, head.Hash())
	assert.NoError(t, err)

	err = reopenedStore.Start(ctx)
	require.NoError(t, err)

	reopenedHead, err := reopenedStore.Head(ctx)
	require.NoError(t, err)

	// check that reopened head changed and the store wasn't reinitialized
	assert.Equal(t, suite.Head().Height(), reopenedHead.Height())
	assert.NotEqual(t, head.Height(), reopenedHead.Height())

	err = reopenedStore.Stop(ctx)
	require.NoError(t, err)
}
