package core

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
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
	subFn    func(call int) (chan BlockEvent, error)
	subCalls int

	// getFn serves GetSignedBlock.
	getFn func(ctx context.Context, height int64) (*SignedBlock, error)
}

func (f *fakeSource) SubscribeNewBlockEvent(context.Context) (chan BlockEvent, error) {
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

func (f *fakeSource) GetSignedBlock(ctx context.Context, height int64) (*SignedBlock, error) {
	f.mu.Lock()
	fn := f.getFn
	f.mu.Unlock()
	if fn == nil {
		return nil, errors.New("fakeSource: getFn not set")
	}
	return fn(ctx, height)
}

// GetSignedBlockFrom satisfies the Fetcher interface; a leaf fakeSource has a
// single source, so it just fetches by height.
func (f *fakeSource) GetSignedBlockFrom(ctx context.Context, ev BlockEvent) (*SignedBlock, error) {
	return f.GetSignedBlock(ctx, ev.Height)
}

func (f *fakeSource) ChainID(context.Context) (string, error) { return f.chainID, f.chainIDErr }

// Verify is unused by MultiSource (which verifies sources via ChainID) but
// required to satisfy the Fetcher interface.
func (f *fakeSource) Verify(context.Context, string) error { return nil }

func (f *fakeSource) IsSyncing(context.Context) (bool, error) { return f.syncing, f.syncingErr }

// IsSyncingFrom satisfies the Fetcher interface; a leaf fakeSource has a
// single source, so it just reports its own status.
func (f *fakeSource) IsSyncingFrom(ctx context.Context, _ BlockEvent) (bool, error) {
	return f.IsSyncing(ctx)
}

func tagged(addr string, f blockSource) taggedSource {
	return taggedSource{fetcher: f, addr: addr}
}

func blockAt(h int64) SignedBlock {
	return SignedBlock{Header: &types.Header{Height: h}}
}

func recv(t *testing.T, ch <-chan BlockEvent) (BlockEvent, bool) {
	t.Helper()
	select {
	case ev, ok := <-ch:
		return ev, ok
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting on channel")
		return BlockEvent{}, false
	}
}

// neverDelivers returns a channel that is never written to or closed, so a
// source "stays subscribed" until ctx is canceled.
func neverDelivers() chan BlockEvent { return make(chan BlockEvent) }

func TestMultiSource_SubscribeFanIn(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	chA := make(chan BlockEvent)
	chB := make(chan BlockEvent)
	srcA := &fakeSource{subFn: func(int) (chan BlockEvent, error) { return chA, nil }}
	srcB := &fakeSource{subFn: func(int) (chan BlockEvent, error) { return chB, nil }}

	ms := newMultiSource(tagged("srcA", srcA), tagged("srcB", srcB))
	out, err := ms.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)

	// Both sources emit the same height (duplicate) plus one extra; MultiSource
	// forwards everything — dedup is the consumer's job.
	chA <- BlockEvent{Height: 1}
	chB <- BlockEvent{Height: 1}
	chA <- BlockEvent{Height: 2}

	got := make([]int64, 0, 3)
	for range 3 {
		ev, ok := recv(t, out)
		require.True(t, ok)
		got = append(got, ev.Height)
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

	src := &fakeSource{subFn: func(int) (chan BlockEvent, error) { return neverDelivers(), nil }}
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

	dead := make(chan BlockEvent)
	live := make(chan BlockEvent)
	deadSrc := &fakeSource{subFn: func(int) (chan BlockEvent, error) { return dead, nil }}
	liveSrc := &fakeSource{subFn: func(int) (chan BlockEvent, error) { return live, nil }}

	ms := newMultiSource(tagged("dead", deadSrc), tagged("live", liveSrc))
	out, err := ms.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)

	// One source ends; the other must keep working and out must stay open.
	close(dead)
	live <- BlockEvent{Height: 9}
	ev, ok := recv(t, out)
	require.True(t, ok)
	assert.Equal(t, int64(9), ev.Height)
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

func TestMultiSource_GetSignedBlockFrom(t *testing.T) {
	ctx := context.Background()

	// announcing source serves the block at index 1.
	t.Run("fetches from the announcing source", func(t *testing.T) {
		wrongSrc := &fakeSource{getFn: func(context.Context, int64) (*SignedBlock, error) {
			return nil, errors.New("not the announcer; must not be called")
		}}
		announcer := &fakeSource{getFn: func(_ context.Context, h int64) (*SignedBlock, error) {
			b := blockAt(h)
			return &b, nil
		}}
		ms := newMultiSource(tagged("wrong", wrongSrc), tagged("announcer", announcer))

		b, err := ms.GetSignedBlockFrom(ctx, BlockEvent{Height: 7, source: 1})
		require.NoError(t, err)
		assert.Equal(t, int64(7), b.Header.Height)
	})

	// SRP: no fallback. If the announcing source fails, the error propagates and
	// the OTHER source is NOT tried — the fan-in re-announces the height and the
	// Listener retries from there.
	t.Run("announcer failure propagates without trying others", func(t *testing.T) {
		var otherCalled atomic.Bool
		other := &fakeSource{getFn: func(context.Context, int64) (*SignedBlock, error) {
			otherCalled.Store(true)
			b := blockAt(9)
			return &b, nil
		}}
		down := &fakeSource{getFn: func(context.Context, int64) (*SignedBlock, error) {
			return nil, errors.New("down")
		}}
		ms := newMultiSource(tagged("other", other), tagged("down", down))

		_, err := ms.GetSignedBlockFrom(ctx, BlockEvent{Height: 9, source: 1}) // source 1 = down
		require.Error(t, err)
		assert.False(t, otherCalled.Load(), "must not fall back to another source")
	})

	t.Run("out-of-range source index errors", func(t *testing.T) {
		ms := newMultiSource(tagged("only", &fakeSource{}))
		_, err := ms.GetSignedBlockFrom(ctx, BlockEvent{Height: 1, source: 5})
		require.Error(t, err)
	})
}

func TestMultiSource_IsSyncingFrom(t *testing.T) {
	ctx := context.Background()

	t.Run("routes to the announcing source", func(t *testing.T) {
		ms := newMultiSource(
			tagged("caught-up", &fakeSource{syncing: false}),
			tagged("catching-up", &fakeSource{syncing: true}),
		)

		// the catching-up source announced: its replayed block must be treated
		// as syncing even though the other source is caught up.
		syncing, err := ms.IsSyncingFrom(ctx, BlockEvent{Height: 7, source: 1})
		require.NoError(t, err)
		assert.True(t, syncing)

		syncing, err = ms.IsSyncingFrom(ctx, BlockEvent{Height: 8, source: 0})
		require.NoError(t, err)
		assert.False(t, syncing)
	})

	t.Run("no fallback: announcing source's error surfaces even if another is fine", func(t *testing.T) {
		ms := newMultiSource(
			tagged("healthy", &fakeSource{syncing: false}),
			tagged("down", &fakeSource{syncingErr: errors.New("down")}),
		)
		_, err := ms.IsSyncingFrom(ctx, BlockEvent{Height: 9, source: 1})
		require.Error(t, err)
	})

	t.Run("out-of-range source index errors", func(t *testing.T) {
		ms := newMultiSource(tagged("only", &fakeSource{}))
		_, err := ms.IsSyncingFrom(ctx, BlockEvent{Height: 1, source: 5})
		require.Error(t, err)
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

	t.Run("unreachable source is pruned even when another is confirmed good", func(t *testing.T) {
		ms := newMultiSource(
			tagged("good", &fakeSource{chainID: "mocha-4"}),
			tagged("down", &fakeSource{chainIDErr: errors.New("unreachable")}),
		)
		require.NoError(t, ms.Verify(ctx, "mocha-4"))
		require.Len(t, ms.sources, 1,
			"a source that cannot vouch for its network must not enter the active set")
		assert.Equal(t, "good", ms.sources[0].addr)
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
// before MultiSource existed: heights pass through unchanged and in order, and
// ChainID/IsSyncing mirror the one source.
func TestMultiSource_SingleSource(t *testing.T) {
	t.Run("forwards every height in order", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		in := make(chan BlockEvent)
		src := &fakeSource{subFn: func(call int) (chan BlockEvent, error) {
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
				in <- BlockEvent{Height: h}
			}
		}()

		got := make([]int64, 0, 5)
		for range 5 {
			ev, ok := recv(t, out)
			require.True(t, ok)
			got = append(got, ev.Height)
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

	t.Run("isSyncingFrom passthrough", func(t *testing.T) {
		ctx := context.Background()
		ev := BlockEvent{Height: 1, source: 0}

		syncing, err := newMultiSource(tagged("only", &fakeSource{syncing: true})).IsSyncingFrom(ctx, ev)
		require.NoError(t, err)
		assert.True(t, syncing)

		syncing, err = newMultiSource(tagged("only", &fakeSource{syncing: false})).IsSyncingFrom(ctx, ev)
		require.NoError(t, err)
		assert.False(t, syncing)

		_, err = newMultiSource(tagged("only", &fakeSource{syncingErr: errors.New("down")})).IsSyncingFrom(ctx, ev)
		require.Error(t, err, "a failing single source must surface as an error, like the bare fetcher")
	})

	t.Run("source subscribe error closes out", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		src := &fakeSource{subFn: func(int) (chan BlockEvent, error) {
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
