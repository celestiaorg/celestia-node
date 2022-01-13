package header

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncSimple(t *testing.T) {
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
	syncer.sync(ctx) // manually init blocking sync instead of Start

	exp, err := remoteStore.Head(ctx)
	require.Nil(t, err)

	have, err := localStore.Head(ctx)
	require.Nil(t, err)
	assert.Equal(t, exp.Height, have.Height)
}

func TestSyncerInitializeSubjectiveHead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	suite := NewTestSuite(t, 3)
	head := suite.Head()

	remoteStore, err := NewStoreWithHead(sync.MutexWrap(datastore.NewMapDatastore()), head)
	require.Nil(t, err)

	err = remoteStore.Append(ctx, suite.GenExtendedHeaders(100)...)
	require.Nil(t, err)

	fakeExchange := NewLocalExchange(remoteStore)

	// we don't the head here and expect syncer to load it himself
	localStore, err := NewStore(sync.MutexWrap(datastore.NewMapDatastore()))
	require.Nil(t, err)

	syncer := NewSyncer(fakeExchange, localStore, nil, head.Hash())
	syncer.sync(ctx) // manually init blocking sync instead of Start

	exp, err := remoteStore.Head(ctx)
	require.Nil(t, err)

	have, err := localStore.Head(ctx)
	require.Nil(t, err)
	assert.Equal(t, exp.Height, have.Height)
}

func TestSyncCatchUp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	suite := NewTestSuite(t, 3)
	head := suite.Head()

	remoteStore, err := NewStoreWithHead(sync.MutexWrap(datastore.NewMapDatastore()), head)
	require.Nil(t, err)

	fakeExchange := NewLocalExchange(remoteStore)

	localStore, err := NewStoreWithHead(sync.MutexWrap(datastore.NewMapDatastore()), head)
	require.Nil(t, err)

	syncer := NewSyncer(fakeExchange, localStore, nil, head.Hash())
	// 1. Initial sync
	err = syncer.Start(ctx)
	require.Nil(t, err)

	// 2. chain grows and syncer misses that
	err = remoteStore.Append(ctx, suite.GenExtendedHeaders(100)...)
	require.Nil(t, err)

	// 3. syncer rcvs header from the future and starts catching-up
	res := syncer.validate(ctx, "", suite.GenExtendedHeaders(1)[0])
	assert.Equal(t, pubsub.ValidationAccept, res)

	// TODO(@Wondertan): Async blocking instead of sleep
	time.Sleep(time.Second)

	exp, err := remoteStore.Head(ctx)
	require.Nil(t, err)

	have, err := localStore.Head(ctx)
	require.Nil(t, err)

	// 4. assert syncer caught-up
	assert.Equal(t, exp.Height+1, have.Height) // plus one as we didn't add last header to remoteStore
}
