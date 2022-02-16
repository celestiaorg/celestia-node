package header

import (
	"context"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncSimpleRequestingHead(t *testing.T) {
	// this way we force local head of Syncer to expire, so it requests a new one from trusted peer
	TrustingPeriod = time.Microsecond
	requestSize = 13 // just some random number

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	suite := NewTestSuite(t, 3)
	head := suite.Head()

	remoteStore := NewTestStore(ctx, t, head)
	err := remoteStore.Append(ctx, suite.GenExtendedHeaders(100)...)
	require.NoError(t, err)

	localStore := NewTestStore(ctx, t, head)
	syncer := NewSyncer(NewLocalExchange(remoteStore), localStore, &DummySubscriber{})
	err = syncer.Start(ctx)
	require.NoError(t, err)

	// TODO(@Wondertan): Async blocking instead of sleep
	time.Sleep(time.Second)

	exp, err := remoteStore.Head(ctx)
	require.NoError(t, err)

	have, err := localStore.Head(ctx)
	require.NoError(t, err)
	assert.Equal(t, exp.Height, have.Height)
	assert.Empty(t, syncer.pending.Head())
}

func TestSyncCatchUp(t *testing.T) {
	// just set a big enough value, so we trust local header and don't request anything
	TrustingPeriod = time.Minute

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	suite := NewTestSuite(t, 3)
	head := suite.Head()

	remoteStore := NewTestStore(ctx, t, head)
	localStore := NewTestStore(ctx, t, head)
	syncer := NewSyncer(NewLocalExchange(remoteStore), localStore, &DummySubscriber{})
	// 1. Initial sync
	err := syncer.Start(ctx)
	require.NoError(t, err)

	// 2. chain grows and syncer misses that
	err = remoteStore.Append(ctx, suite.GenExtendedHeaders(100)...)
	require.NoError(t, err)

	// 3. syncer rcvs header from the future and starts catching-up
	res := syncer.processIncoming(ctx, suite.GenExtendedHeaders(1)[0])
	assert.Equal(t, pubsub.ValidationAccept, res)

	_, err = localStore.GetByHeight(ctx, 102)
	require.NoError(t, err)

	exp, err := remoteStore.Head(ctx)
	require.NoError(t, err)

	have, err := localStore.Head(ctx)
	require.NoError(t, err)

	// 4. assert syncer caught-up
	assert.Equal(t, exp.Height+1, have.Height) // plus one as we didn't add last header to remoteStore
	assert.Empty(t, syncer.pending.Head())
}

func TestSyncPendingRangesWithMisses(t *testing.T) {
	// just set a big enough value, so we trust local header and don't request anything
	TrustingPeriod = time.Minute

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	suite := NewTestSuite(t, 3)
	head := suite.Head()

	remoteStore := NewTestStore(ctx, t, head)
	localStore := NewTestStore(ctx, t, head)
	syncer := NewSyncer(NewLocalExchange(remoteStore), localStore, &DummySubscriber{})
	err := syncer.Start(ctx)
	require.NoError(t, err)

	// miss 1 (helps to test that Syncer properly requests missed Headers from Exchange)
	err = remoteStore.Append(ctx, suite.GenExtendedHeaders(1)...)
	require.NoError(t, err)

	range1 := suite.GenExtendedHeaders(15)
	err = remoteStore.Append(ctx, range1...)
	require.NoError(t, err)

	// miss 2
	err = remoteStore.Append(ctx, suite.GenExtendedHeaders(3)...)
	require.NoError(t, err)

	range2 := suite.GenExtendedHeaders(23)
	err = remoteStore.Append(ctx, range2...)
	require.NoError(t, err)

	// manually add to pending
	for _, h := range append(range1, range2...) {
		syncer.pending.Add(h)
	}

	// and fire app a sync
	syncer.sync(ctx)

	exp, err := remoteStore.Head(ctx)
	require.NoError(t, err)

	have, err := localStore.Head(ctx)
	require.NoError(t, err)

	assert.Equal(t, exp.Height, have.Height)
	assert.Empty(t, syncer.pending.Head()) // assert all cache from pending is used
}
