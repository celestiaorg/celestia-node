package header

import (
	"context"
	"testing"

	tmrand "github.com/celestiaorg/celestia-core/libs/rand"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	// Alter Cache sizes to read some values from datastore instead of only cache.
	DefaultStoreCache, DefaultIndexCache = 5, 5

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	suite := NewTestSuite(t, 3)

	ds := sync.MutexWrap(datastore.NewMapDatastore())
	store, err := NewStore(ds)
	require.Nil(t, err)

	err = store.Open(ctx)
	assert.Equal(t, ErrNoHead, err) // check that we can't open a Store without Head.

	store, err = NewStoreWithHead(ds, suite.Head())
	require.NoError(t, err)
	err = store.Open(ctx)
	require.NoError(t, err)

	head, err := store.Head(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, suite.Head(), head)

	in := suite.GenExtendedHeaders(10)
	err = store.Append(ctx, in...)
	require.NoError(t, err)

	out, err := store.GetRangeByHeight(ctx, 1, 11)
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
}
