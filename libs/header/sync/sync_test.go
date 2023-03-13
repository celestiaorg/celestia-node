package sync

import (
	"context"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/libs/header/local"
	"github.com/celestiaorg/celestia-node/libs/header/store"
	"github.com/celestiaorg/celestia-node/libs/header/test"
)

func TestSyncSimpleRequestingHead(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	suite := test.NewTestSuite(t)
	head := suite.Head()

	remoteStore := store.NewTestStore(ctx, t, head)
	err := remoteStore.Append(ctx, suite.GenDummyHeaders(100)...)
	require.NoError(t, err)

	_, err = remoteStore.GetByHeight(ctx, 100)
	require.NoError(t, err)

	localStore := store.NewTestStore(ctx, t, head)
	syncer, err := NewSyncer[*test.DummyHeader](
		local.NewExchange(remoteStore),
		localStore,
		&test.DummySubscriber{},
		WithBlockTime(time.Second*30),
		WithTrustingPeriod(time.Microsecond),
	)
	require.NoError(t, err)
	err = syncer.Start(ctx)
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 10) // needs some to realize it is syncing
	err = syncer.SyncWait(ctx)
	require.NoError(t, err)

	exp, err := remoteStore.Head(ctx)
	require.NoError(t, err)

	have, err := localStore.Head(ctx)
	require.NoError(t, err)
	assert.Equal(t, exp.Height(), have.Height())
	assert.Empty(t, syncer.pending.Head())

	state := syncer.State()
	assert.Equal(t, uint64(exp.Height()), state.Height)
	assert.Equal(t, uint64(2), state.FromHeight)
	assert.Equal(t, uint64(exp.Height()), state.ToHeight)
	assert.True(t, state.Finished(), state)
}

func TestDoSyncFullRangeFromExternalPeer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	suite := test.NewTestSuite(t)
	head := suite.Head()

	remoteStore := store.NewTestStore(ctx, t, head)
	localStore := store.NewTestStore(ctx, t, head)
	syncer, err := NewSyncer[*test.DummyHeader](
		local.NewExchange(remoteStore),
		localStore,
		&test.DummySubscriber{},
	)
	require.NoError(t, err)
	require.NoError(t, syncer.Start(ctx))

	err = remoteStore.Append(ctx, suite.GenDummyHeaders(int(header.MaxRangeRequestSize))...)
	require.NoError(t, err)
	// give store time to update heightSub index
	time.Sleep(time.Millisecond * 100)

	localHead, err := localStore.Head(ctx)
	require.NoError(t, err)

	remoteHead, err := remoteStore.Head(ctx)
	require.NoError(t, err)

	err = syncer.doSync(ctx, localHead, remoteHead)
	require.NoError(t, err)

	newHead := *syncer.syncedHead.Load()
	require.NoError(t, err)
	require.Equal(t, newHead.Height(), remoteHead.Height())
}

func TestSyncCatchUp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	suite := test.NewTestSuite(t)
	head := suite.Head()

	remoteStore := store.NewTestStore(ctx, t, head)
	localStore := store.NewTestStore(ctx, t, head)
	syncer, err := NewSyncer[*test.DummyHeader](
		local.NewExchange(remoteStore),
		localStore,
		&test.DummySubscriber{},
		WithTrustingPeriod(time.Minute),
	)
	require.NoError(t, err)
	// 1. Initial sync
	err = syncer.Start(ctx)
	require.NoError(t, err)

	// 2. chain grows and syncer misses that
	err = remoteStore.Append(ctx, suite.GenDummyHeaders(100)...)
	require.NoError(t, err)

	incomingHead := suite.GenDummyHeaders(1)[0]
	// 3. syncer rcvs header from the future and starts catching-up
	res := syncer.incomingNetHead(ctx, incomingHead)
	assert.Equal(t, pubsub.ValidationAccept, res)

	time.Sleep(time.Millisecond * 10) // needs some to realize it is syncing
	err = syncer.SyncWait(ctx)
	require.NoError(t, err)
	exp, err := remoteStore.Head(ctx)
	require.NoError(t, err)

	// 4. assert syncer caught-up
	have, err := localStore.Head(ctx)
	require.NoError(t, err)
	headerPtr := syncer.syncedHead.Load()
	require.NotNil(t, headerPtr)
	head = *headerPtr
	assert.Equal(t, head.Height(), incomingHead.Height())
	assert.Equal(t, exp.Height()+1, have.Height()) // plus one as we didn't add last header to remoteStore
	assert.Empty(t, syncer.pending.Head())

	state := syncer.State()
	assert.Equal(t, uint64(exp.Height()+1), state.Height)
	assert.Equal(t, uint64(2), state.FromHeight)
	assert.Equal(t, uint64(exp.Height()+1), state.ToHeight)
	assert.True(t, state.Finished(), state)
}

func TestSyncPendingRangesWithMisses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	suite := test.NewTestSuite(t)
	head := suite.Head()

	remoteStore := store.NewTestStore(ctx, t, head)
	localStore := store.NewTestStore(ctx, t, head)
	syncer, err := NewSyncer[*test.DummyHeader](
		local.NewExchange(remoteStore),
		localStore,
		&test.DummySubscriber{},
		WithTrustingPeriod(time.Minute),
	)
	require.NoError(t, err)
	err = syncer.Start(ctx)
	require.NoError(t, err)

	// miss 1 (helps to test that Syncer properly requests missed Headers from Exchange)
	err = remoteStore.Append(ctx, suite.GenDummyHeaders(1)...)
	require.NoError(t, err)

	range1 := suite.GenDummyHeaders(15)
	err = remoteStore.Append(ctx, range1...)
	require.NoError(t, err)

	// miss 2
	err = remoteStore.Append(ctx, suite.GenDummyHeaders(3)...)
	require.NoError(t, err)

	range2 := suite.GenDummyHeaders(23)
	err = remoteStore.Append(ctx, range2...)
	require.NoError(t, err)

	// manually add to pending
	for _, h := range append(range1, range2...) {
		syncer.pending.Add(h)
	}

	// and fire up a sync
	syncer.sync(ctx)

	_, err = remoteStore.GetByHeight(ctx, 43)
	require.NoError(t, err)
	_, err = localStore.GetByHeight(ctx, 43)
	require.NoError(t, err)

	headerPtr := syncer.syncedHead.Load()
	require.NotNil(t, headerPtr)
	lastHead := *headerPtr
	require.Equal(t, lastHead.Height(), int64(43))
	exp, err := remoteStore.Head(ctx)
	require.NoError(t, err)

	have, err := localStore.Head(ctx)
	require.NoError(t, err)

	assert.Equal(t, exp.Height(), have.Height())
	assert.Empty(t, syncer.pending.Head()) // assert all cache from pending is used
}

// Test that only one objective header is requested at a time
func TestSyncer_OnlyOneRecentRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	suite := test.NewTestSuite(t)
	store := store.NewTestStore(ctx, t, suite.Head())
	newHead := suite.GetRandomHeader()
	exchange := &exchangeCountingHead{header: newHead}
	syncer, err := NewSyncer[*test.DummyHeader](exchange, store, &test.DummySubscriber{}, WithBlockTime(time.Nanosecond))
	require.NoError(t, err)

	res := make(chan *test.DummyHeader)
	for i := 0; i < 10; i++ {
		go func() {
			head, err := syncer.networkHead(ctx)
			if err != nil {
				panic(err)
			}
			select {
			case res <- head:
			case <-ctx.Done():
				return
			}
		}()
	}

	for i := 0; i < 10; i++ {
		head := <-res
		assert.Equal(t, exchange.header, head)
	}
	assert.Equal(t, 1, exchange.counter)
}

// TestSyncer_FindHeadersReturnsCorrectRange ensures that `findHeaders` returns
// range [from;to]
func TestSyncer_FindHeadersReturnsCorrectRange(t *testing.T) {
	// Test consists of 3 steps:
	// 1. get range of headers from pending; [2;11]
	// 2. get headers from the remote store; [12;20]
	// 3. apply last header from pending;
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	suite := test.NewTestSuite(t)
	head := suite.Head()

	remoteStore := store.NewTestStore(ctx, t, head)
	localStore := store.NewTestStore(ctx, t, head)
	syncer, err := NewSyncer[*test.DummyHeader](
		local.NewExchange(remoteStore),
		localStore,
		&test.DummySubscriber{},
	)
	require.NoError(t, err)

	range1 := suite.GenDummyHeaders(10)
	// manually add to pending
	for _, h := range range1 {
		syncer.pending.Add(h)
	}
	err = remoteStore.Append(ctx, range1...)
	require.NoError(t, err)
	err = remoteStore.Append(ctx, suite.GenDummyHeaders(9)...)
	require.NoError(t, err)

	syncer.pending.Add(suite.GetRandomHeader())
	require.NoError(t, err)
	err = syncer.processHeaders(ctx, head, 21)
	require.NoError(t, err)

	headerPtr := syncer.syncedHead.Load()
	require.NotNil(t, headerPtr)
	assert.Equal(t, (*headerPtr).Height(), int64(21))
}

type exchangeCountingHead struct {
	header  *test.DummyHeader
	counter int
}

func (e *exchangeCountingHead) Head(context.Context) (*test.DummyHeader, error) {
	e.counter++
	time.Sleep(time.Millisecond * 100) // simulate requesting something
	return e.header, nil
}

func (e *exchangeCountingHead) Get(ctx context.Context, bytes header.Hash) (*test.DummyHeader, error) {
	panic("implement me")
}

func (e *exchangeCountingHead) GetByHeight(ctx context.Context, u uint64) (*test.DummyHeader, error) {
	panic("implement me")
}

func (e *exchangeCountingHead) GetRangeByHeight(
	c context.Context,
	from, amount uint64,
) ([]*test.DummyHeader, error) {
	panic("implement me")
}

func (e *exchangeCountingHead) GetVerifiedRange(c context.Context, from *test.DummyHeader, amount uint64,
) ([]*test.DummyHeader, error) {
	panic("implement me")
}
