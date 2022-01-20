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
	"github.com/tendermint/tendermint/libs/bytes"
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

	// we don't load the head here and expect syncer to load it himself
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
	res := syncer.incoming(ctx, "", suite.GenExtendedHeaders(1)[0])
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

func TestSyncPendingRangesWithMisses(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	suite := NewTestSuite(t, 3)
	head := suite.Head()

	remoteStore, err := NewStoreWithHead(sync.MutexWrap(datastore.NewMapDatastore()), head)
	require.Nil(t, err)

	fakeExchange := NewLocalExchange(remoteStore)

	localStore, err := NewStoreWithHead(sync.MutexWrap(datastore.NewMapDatastore()), head)
	require.Nil(t, err)

	syncer := NewSyncer(&delayedExchange{fakeExchange}, localStore, nil, head.Hash())
	err = syncer.Start(ctx)
	require.Nil(t, err)

	// miss 1
	err = remoteStore.Append(ctx, suite.GenExtendedHeaders(1)...)
	require.Nil(t, err)

	range1 := suite.GenExtendedHeaders(15)
	err = remoteStore.Append(ctx, range1...)
	require.Nil(t, err)

	// miss 2
	err = remoteStore.Append(ctx, suite.GenExtendedHeaders(3)...)
	require.Nil(t, err)

	range2 := suite.GenExtendedHeaders(23)
	err = remoteStore.Append(ctx, range2...)
	require.Nil(t, err)

	for _, h := range append(range1, range2...) {
		res := syncer.incoming(ctx, "", h)
		assert.Equal(t, pubsub.ValidationAccept, res)
	}

	// TODO(@Wondertan): Async blocking instead of sleep
	time.Sleep(time.Second)

	exp, err := remoteStore.Head(ctx)
	require.Nil(t, err)

	have, err := localStore.Head(ctx)
	require.Nil(t, err)

	assert.Equal(t, exp.Height, have.Height)
	assert.Empty(t, syncer.pending.Head())
}

// delayedExchange prevents sync from finishing instantly
type delayedExchange struct {
	innner Exchange
}

func (d *delayedExchange) delay(ctx context.Context) {
	select {
	case <-time.After(time.Millisecond * 100):
	case <-ctx.Done():
	}
}

func (d *delayedExchange) RequestHead(ctx context.Context) (*ExtendedHeader, error) {
	d.delay(ctx)
	return d.innner.RequestHead(ctx)
}

func (d *delayedExchange) RequestHeader(ctx context.Context, height uint64) (*ExtendedHeader, error) {
	d.delay(ctx)
	return d.innner.RequestHeader(ctx, height)
}

func (d *delayedExchange) RequestHeaders(ctx context.Context, origin, amount uint64) ([]*ExtendedHeader, error) {
	d.delay(ctx)
	return d.innner.RequestHeaders(ctx, origin, amount)
}

func (d *delayedExchange) RequestByHash(ctx context.Context, hash bytes.HexBytes) (*ExtendedHeader, error) {
	d.delay(ctx)
	return d.innner.RequestByHash(ctx, hash)
}
