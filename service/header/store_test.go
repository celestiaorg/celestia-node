package header

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmrand "github.com/tendermint/tendermint/libs/rand"
)

func TestStore(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	suite := NewTestSuite(t, 3)

	ds := sync.MutexWrap(datastore.NewMapDatastore())
	store, err := NewStoreWithHead(ctx, ds, suite.Head())
	require.NoError(t, err)

	err = store.Start(ctx)
	require.NoError(t, err)

	head, err := store.Head(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, suite.Head().Hash(), head.Hash())

	in := suite.GenExtendedHeaders(10)
	ln, err := store.Append(ctx, in...)
	require.NoError(t, err)
	assert.Equal(t, 10, ln)

	out, err := store.GetRangeByHeight(ctx, 2, 12)
	require.NoError(t, err)
	for i, h := range in {
		assert.Equal(t, h.Hash(), out[i].Hash())
	}

	head, err = store.Head(ctx)
	require.NoError(t, err)
	assert.Equal(t, out[len(out)-1].Hash(), head.Hash())

	ok, err := store.Has(ctx, in[5].Hash())
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = store.Has(ctx, tmrand.Bytes(32))
	require.NoError(t, err)
	assert.False(t, ok)

	go func() {
		ln, err := store.Append(ctx, suite.GenExtendedHeaders(1)...)
		require.NoError(t, err)
		assert.Equal(t, 1, ln)
	}()

	h, err := store.GetByHeight(ctx, 12)
	require.NoError(t, err)
	assert.NotNil(t, h)

	err = store.Stop(ctx)
	require.NoError(t, err)

	// check that the store can be successfully started after previous stop
	// with all data being flushed.
	store, err = NewStore(ds)
	require.NoError(t, err)

	err = store.Start(ctx)
	require.NoError(t, err)

	head, err = store.Head(ctx)
	require.NoError(t, err)
	assert.Equal(t, suite.Head().Hash(), head.Hash())

	out, err = store.GetRangeByHeight(ctx, 1, 13)
	require.NoError(t, err)
	assert.Len(t, out, 12)

	err = store.Stop(ctx)
	require.NoError(t, err)
}
