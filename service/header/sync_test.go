package header

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSync(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	suite := NewTestSuite(t, 3)
	head := suite.Head()

	remoteStore, err := NewStoreWithHead(sync.MutexWrap(datastore.NewMapDatastore()), head)
	require.Nil(t, err)

	err = remoteStore.Append(ctx, suite.GenExtendedHeaders(100)...)
	require.Nil(t, err)

	fakeExchange := NewLocalExchange(remoteStore)

	localStore, err := NewStoreWithHead(sync.MutexWrap(datastore.NewMapDatastore()), head)
	require.Nil(t, err)

	requestSize = 13 // just some random number
	syncer := NewSyncer(fakeExchange, localStore, nil, head.Hash())
	syncer.sync(ctx)

	exp, err := remoteStore.Head(ctx)
	require.Nil(t, err)

	have, err := localStore.Head(ctx)
	require.Nil(t, err)
	assert.Equal(t, exp.Height, have.Height)
}
