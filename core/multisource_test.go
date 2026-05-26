package core

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSource is an in-memory Fetcher used to drive MultiSource without a live
// gRPC connection.
type fakeSource struct {
	mu sync.Mutex

	chainID    string
	chainIDErr error

	syncing    bool
	syncingErr error

	// subFn produces the channel/error for each SubscribeNewBlockEvent call,
	// indexed by call number, to simulate (re)subscription.
	subFn    func(call int) (chan SignedBlock, error)
	subCalls int
}

func (f *fakeSource) SubscribeNewBlockEvent(context.Context) (chan SignedBlock, error) {
	f.mu.Lock()
	call := f.subCalls
	f.subCalls++
	fn := f.subFn
	f.mu.Unlock()
	if fn == nil {
		return nil, errors.New("fakeSource: subFn not set")
	}
	return fn(call)
}

func (f *fakeSource) ChainID(context.Context) (string, error) { return f.chainID, f.chainIDErr }

// Verify is unused by MultiSource (which verifies sources via ChainID) but
// required to satisfy the Fetcher interface.
func (f *fakeSource) Verify(context.Context, string) error { return nil }

func (f *fakeSource) IsSyncing(context.Context) (bool, error) { return f.syncing, f.syncingErr }

func tagged(addr string, f Fetcher) taggedSource {
	return taggedSource{fetcher: f, addr: addr}
}

func blockAt(h int64) SignedBlock {
	return SignedBlock{Header: &types.Header{Height: h}}
}

func recv(t *testing.T, ch <-chan SignedBlock) (SignedBlock, bool) {
	t.Helper()
	select {
	case b, ok := <-ch:
		return b, ok
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting on channel")
		return SignedBlock{}, false
	}
}

// neverDelivers returns a channel that is never written to or closed, so a
// source "stays subscribed" until ctx is canceled.
func neverDelivers() chan SignedBlock { return make(chan SignedBlock) }

func TestMultiSource_SubscribeFanIn(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	chA := make(chan SignedBlock)
	chB := make(chan SignedBlock)
	srcA := &fakeSource{subFn: func(int) (chan SignedBlock, error) { return chA, nil }}
	srcB := &fakeSource{subFn: func(int) (chan SignedBlock, error) { return chB, nil }}

	ms := newMultiSource(tagged("srcA", srcA), tagged("srcB", srcB))
	out, err := ms.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)

	// Both sources emit the same height (duplicate) plus one extra; MultiSource
	// forwards everything — dedup is the consumer's job.
	chA <- blockAt(1)
	chB <- blockAt(1)
	chA <- blockAt(2)

	got := make([]int64, 0, 3)
	for range 3 {
		b, ok := recv(t, out)
		require.True(t, ok)
		got = append(got, b.Header.Height)
	}
	slices.Sort(got)
	assert.Equal(t, []int64{1, 1, 2}, got)

	// Canceling drains the sources and closes the fan-in channel.
	cancel()
	_, ok := recv(t, out)
	assert.False(t, ok, "out channel should close after ctx cancel")
}

func TestMultiSource_CancelClosesOut(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	src := &fakeSource{subFn: func(int) (chan SignedBlock, error) { return neverDelivers(), nil }}
	ms := newMultiSource(tagged("src", src))

	out, err := ms.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)

	cancel()
	_, ok := recv(t, out)
	assert.False(t, ok, "out channel should close after ctx cancel")
}

// TestMultiSource_ClosedSourceDoesNotCloseOut verifies that one source ending
// its subscription does not close the fan-in channel: the surviving source
// keeps delivering, and out closes only once every source goroutine has exited.
func TestMultiSource_ClosedSourceDoesNotCloseOut(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	dead := make(chan SignedBlock)
	live := make(chan SignedBlock)
	deadSrc := &fakeSource{subFn: func(int) (chan SignedBlock, error) { return dead, nil }}
	liveSrc := &fakeSource{subFn: func(int) (chan SignedBlock, error) { return live, nil }}

	ms := newMultiSource(tagged("dead", deadSrc), tagged("live", liveSrc))
	out, err := ms.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)

	// One source ends; the other must keep working and out must stay open.
	close(dead)
	live <- blockAt(9)
	b, ok := recv(t, out)
	require.True(t, ok)
	assert.Equal(t, int64(9), b.Header.Height)
}

func TestMultiSource_ChainID(t *testing.T) {
	ctx := context.Background()

	t.Run("first source responds", func(t *testing.T) {
		ms := newMultiSource(tagged("a", &fakeSource{chainID: "arabica-11"}))
		id, err := ms.ChainID(ctx)
		require.NoError(t, err)
		assert.Equal(t, "arabica-11", id)
	})

	t.Run("falls back past a failing source", func(t *testing.T) {
		ms := newMultiSource(
			tagged("a", &fakeSource{chainIDErr: errors.New("down")}),
			tagged("b", &fakeSource{chainID: "mocha-4"}),
		)
		id, err := ms.ChainID(ctx)
		require.NoError(t, err)
		assert.Equal(t, "mocha-4", id)
	})

	t.Run("errors when all sources fail", func(t *testing.T) {
		ms := newMultiSource(
			tagged("a", &fakeSource{chainIDErr: errors.New("a")}),
			tagged("b", &fakeSource{chainIDErr: errors.New("b")}),
		)
		_, err := ms.ChainID(ctx)
		require.Error(t, err)
	})
}

func TestMultiSource_IsSyncing(t *testing.T) {
	ctx := context.Background()

	t.Run("not syncing if any source is caught up", func(t *testing.T) {
		ms := newMultiSource(
			tagged("a", &fakeSource{syncing: true}),
			tagged("b", &fakeSource{syncing: false}), // caught up
		)
		syncing, err := ms.IsSyncing(ctx)
		require.NoError(t, err)
		assert.False(t, syncing)
	})

	t.Run("syncing if every source is syncing", func(t *testing.T) {
		ms := newMultiSource(
			tagged("a", &fakeSource{syncing: true}),
			tagged("b", &fakeSource{syncing: true}),
		)
		syncing, err := ms.IsSyncing(ctx)
		require.NoError(t, err)
		assert.True(t, syncing)
	})

	t.Run("a single reachable syncing source wins over an errored one", func(t *testing.T) {
		ms := newMultiSource(
			tagged("a", &fakeSource{syncingErr: errors.New("down")}),
			tagged("b", &fakeSource{syncing: true}),
		)
		syncing, err := ms.IsSyncing(ctx)
		require.NoError(t, err)
		assert.True(t, syncing)
	})

	t.Run("errors only when no source is reachable", func(t *testing.T) {
		ms := newMultiSource(
			tagged("a", &fakeSource{syncingErr: errors.New("a")}),
			tagged("b", &fakeSource{syncingErr: errors.New("b")}),
		)
		syncing, err := ms.IsSyncing(ctx)
		require.Error(t, err)
		assert.True(t, syncing, "defaults to syncing when unknown")
	})
}

func TestMultiSource_Verify(t *testing.T) {
	ctx := context.Background()

	t.Run("empty expected is an error: a node must know its network", func(t *testing.T) {
		ms := newMultiSource(tagged("a", &fakeSource{chainID: "mocha-4"}))
		require.Error(t, ms.Verify(ctx, ""))
	})

	t.Run("all on expected chain are kept", func(t *testing.T) {
		ms := newMultiSource(
			tagged("a", &fakeSource{chainID: "mocha-4"}),
			tagged("b", &fakeSource{chainID: "mocha-4"}),
		)
		require.NoError(t, ms.Verify(ctx, "mocha-4"))
		assert.Len(t, ms.sources, 2)
	})

	t.Run("wrong-chain source is pruned, others kept", func(t *testing.T) {
		ms := newMultiSource(
			tagged("good", &fakeSource{chainID: "mocha-4"}),
			tagged("bad", &fakeSource{chainID: "arabica-11"}),
		)
		require.NoError(t, ms.Verify(ctx, "mocha-4"))
		require.Len(t, ms.sources, 1)
		assert.Equal(t, "good", ms.sources[0].addr)
	})

	t.Run("unreachable source is kept when another is confirmed good", func(t *testing.T) {
		ms := newMultiSource(
			tagged("good", &fakeSource{chainID: "mocha-4"}),
			tagged("down", &fakeSource{chainIDErr: errors.New("unreachable")}),
		)
		require.NoError(t, ms.Verify(ctx, "mocha-4"))
		assert.Len(t, ms.sources, 2, "unreachable source kept so it can join later")
	})

	t.Run("errors when every reachable source is on the wrong chain", func(t *testing.T) {
		ms := newMultiSource(
			tagged("a", &fakeSource{chainID: "arabica-11"}),
			tagged("b", &fakeSource{chainID: "arabica-11"}),
		)
		err := ms.Verify(ctx, "mocha-4")
		require.Error(t, err)
		assert.Empty(t, ms.sources, "all wrong-chain sources pruned")
	})

	t.Run("errors when no source could be reached", func(t *testing.T) {
		ms := newMultiSource(
			tagged("a", &fakeSource{chainIDErr: errors.New("down")}),
			tagged("b", &fakeSource{chainIDErr: errors.New("down")}),
		)
		err := ms.Verify(ctx, "mocha-4")
		require.Error(t, err, "cannot start without confirming at least one source")
	})
}

// TestMultiSource_SingleSource pins the contract that a MultiSource wrapping a
// single endpoint behaves like the bare BlockFetcher the Listener consumed
// before MultiSource existed: blocks pass through unchanged and in order, and
// ChainID/IsSyncing mirror the one source.
func TestMultiSource_SingleSource(t *testing.T) {
	t.Run("forwards every block in order", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		in := make(chan SignedBlock)
		src := &fakeSource{subFn: func(call int) (chan SignedBlock, error) {
			if call == 0 {
				return in, nil
			}
			return neverDelivers(), nil
		}}
		ms := newMultiSource(tagged("only", src))

		out, err := ms.SubscribeNewBlockEvent(ctx)
		require.NoError(t, err)

		// A single source has no fan-in interleaving, so order must be preserved
		// exactly as delivered — same as reading the BlockFetcher channel directly.
		go func() {
			for h := int64(1); h <= 5; h++ {
				in <- blockAt(h)
			}
		}()

		got := make([]int64, 0, 5)
		for range 5 {
			b, ok := recv(t, out)
			require.True(t, ok)
			got = append(got, b.Header.Height)
		}
		assert.Equal(t, []int64{1, 2, 3, 4, 5}, got, "single source must preserve delivery order")
	})

	t.Run("chainID passthrough", func(t *testing.T) {
		ctx := context.Background()

		id, err := newMultiSource(tagged("only", &fakeSource{chainID: "arabica-11"})).ChainID(ctx)
		require.NoError(t, err)
		assert.Equal(t, "arabica-11", id)

		_, err = newMultiSource(tagged("only", &fakeSource{chainIDErr: errors.New("down")})).ChainID(ctx)
		require.Error(t, err, "a failing single source must surface as an error, like the bare fetcher")
	})

	t.Run("isSyncing passthrough", func(t *testing.T) {
		ctx := context.Background()

		syncing, err := newMultiSource(tagged("only", &fakeSource{syncing: true})).IsSyncing(ctx)
		require.NoError(t, err)
		assert.True(t, syncing)

		syncing, err = newMultiSource(tagged("only", &fakeSource{syncing: false})).IsSyncing(ctx)
		require.NoError(t, err)
		assert.False(t, syncing)

		// On error the bare BlockFetcher returns (false, err) while MultiSource
		// returns (true, err); the bool is irrelevant because handleNewSignedBlock
		// bails on err != nil before reading it. What matters is that the error
		// still propagates, which it does.
		_, err = newMultiSource(tagged("only", &fakeSource{syncingErr: errors.New("down")})).IsSyncing(ctx)
		require.Error(t, err)
	})

	t.Run("source subscribe error closes out", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		src := &fakeSource{subFn: func(int) (chan SignedBlock, error) {
			return nil, errors.New("subscribe boom")
		}}
		ms := newMultiSource(tagged("only", src))

		out, err := ms.SubscribeNewBlockEvent(ctx)
		require.NoError(t, err)

		// The lone source can't subscribe, so its goroutine exits and out closes —
		// the consumer (Listener.listen) then sees the closed channel and stops,
		// matching the single-fetcher end-of-stream path.
		_, ok := recv(t, out)
		assert.False(t, ok, "out should close when the only source cannot subscribe")
	})
}
