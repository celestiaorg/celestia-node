package headers

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	dsbadger "github.com/ipfs/go-ds-badger4"

	libhead_store "github.com/celestiaorg/go-header/store"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
)

// fakeExchange serves a fixed slice of generated headers by height. It supports
// per-call hooks for injecting latency, errors, and ordering tests.
type fakeExchange struct {
	headers []*header.ExtendedHeader
	chainID string

	getByHeightHook       func(ctx context.Context, height uint64, call uint64) error
	getRangeByHeightHook  func(ctx context.Context, from *header.ExtendedHeader, to uint64, call uint64) error
	getByHeightCalls      atomic.Uint64
	getRangeByHeightCalls atomic.Uint64
}

func (f *fakeExchange) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	call := f.getByHeightCalls.Add(1)
	if f.getByHeightHook != nil {
		if err := f.getByHeightHook(ctx, height, call); err != nil {
			return nil, err
		}
	}
	if height == 0 || height > uint64(len(f.headers)) {
		return nil, fmt.Errorf("fake: height %d out of range", height)
	}
	return f.headers[height-1], nil
}

func (f *fakeExchange) GetRangeByHeight(
	ctx context.Context,
	from *header.ExtendedHeader,
	to uint64,
) ([]*header.ExtendedHeader, error) {
	call := f.getRangeByHeightCalls.Add(1)
	if f.getRangeByHeightHook != nil {
		if err := f.getRangeByHeightHook(ctx, from, to, call); err != nil {
			return nil, err
		}
	}
	start := from.Height() + 1
	end := to - 1
	if start > end || end > uint64(len(f.headers)) {
		return nil, fmt.Errorf("fake: bad range %d..%d", start, end)
	}
	return f.headers[start-1 : end], nil
}

// newReplicationFixture spins up a badger-backed header store seeded with the
// first `seed` headers from `all`, plus a fake exchange that can serve any
// height in `all`.
func newReplicationFixture(t *testing.T, all []*header.ExtendedHeader, seed int) (
	*libhead_store.Store[*header.ExtendedHeader],
	func(),
) {
	t.Helper()
	dir := t.TempDir()
	ds, err := dsbadger.NewDatastore(filepath.Join(dir, "data"), nil)
	if err != nil {
		t.Fatalf("open badger: %v", err)
	}
	hstore, err := libhead_store.NewStore[*header.ExtendedHeader](
		ds,
		libhead_store.WithWriteBatchSize(64),
	)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := hstore.Start(context.Background()); err != nil {
		t.Fatalf("start store: %v", err)
	}
	if seed > 0 {
		if err := hstore.Append(context.Background(), all[:seed]...); err != nil {
			t.Fatalf("seed append: %v", err)
		}
	}
	cleanup := func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = hstore.Stop(stopCtx)
		_ = ds.Close()
	}
	return hstore, cleanup
}

// TestFetchLoopAppendsInOrderDespiteOutOfOrderCompletion delays the first
// chunk so chunks complete out of dispatch order. The writer must still
// produce a contiguous append from startHeight..targetHeight.
func TestFetchLoopAppendsInOrderDespiteOutOfOrderCompletion(t *testing.T) {
	suite := headertest.NewTestSuite(t, 3, 0)
	all := suite.GenExtendedHeaders(200)
	chainID := all[0].ChainID()

	fx := &fakeExchange{headers: all, chainID: chainID}
	// First range call lags behind so a later one (higher index) lands first.
	fx.getRangeByHeightHook = func(_ context.Context, _ *header.ExtendedHeader, _ uint64, call uint64) error {
		if call == 1 {
			time.Sleep(120 * time.Millisecond)
		}
		return nil
	}

	hstore, cleanup := newReplicationFixture(t, all, 0)
	defer cleanup()

	last, err := replicateHeaderRange(
		context.Background(), fx, hstore,
		1, 200, 4, time.Second, chainID, nil,
	)
	if err != nil {
		t.Fatalf("replicate: %v", err)
	}
	if last == nil || last.Height() != 200 {
		t.Fatalf("last appended = %v, want height 200", last)
	}
	for h := uint64(1); h <= 200; h++ {
		got, err := hstore.GetByHeight(context.Background(), h)
		if err != nil {
			t.Fatalf("get height %d: %v", h, err)
		}
		if got.Height() != h {
			t.Fatalf("get height %d returned height %d", h, got.Height())
		}
	}
}

// TestFetchLoopRetriesUntilSuccess proves indefinite constant-delay retry for
// transient errors.
func TestFetchLoopRetriesUntilSuccess(t *testing.T) {
	suite := headertest.NewTestSuite(t, 3, 0)
	all := suite.GenExtendedHeaders(100)
	chainID := all[0].ChainID()

	fx := &fakeExchange{headers: all, chainID: chainID}
	// First two attempts on the first range call fail; subsequent ones succeed.
	var rangeFails atomic.Uint64
	fx.getRangeByHeightHook = func(_ context.Context, _ *header.ExtendedHeader, _ uint64, _ uint64) error {
		if rangeFails.Load() < 2 {
			rangeFails.Add(1)
			return errors.New("transient")
		}
		return nil
	}

	hstore, cleanup := newReplicationFixture(t, all, 1) // seed height 1 as local anchor
	defer cleanup()

	last, err := replicateHeaderRange(
		context.Background(), fx, hstore,
		2, 100, 1, 200*time.Millisecond, chainID, nil,
	)
	if err != nil {
		t.Fatalf("replicate: %v", err)
	}
	if last == nil || last.Height() != 100 {
		t.Fatalf("last appended = %v, want height 100", last)
	}
	if rangeFails.Load() != 2 {
		t.Fatalf("expected 2 failures before success, got %d", rangeFails.Load())
	}
}

// TestFetchLoopCancelStopsRetry asserts ctx cancel breaks out of the indefinite
// retry loop.
func TestFetchLoopCancelStopsRetry(t *testing.T) {
	suite := headertest.NewTestSuite(t, 3, 0)
	all := suite.GenExtendedHeaders(50)
	chainID := all[0].ChainID()

	fx := &fakeExchange{headers: all, chainID: chainID}
	fx.getRangeByHeightHook = func(_ context.Context, _ *header.ExtendedHeader, _ uint64, _ uint64) error {
		return errors.New("always fails")
	}

	hstore, cleanup := newReplicationFixture(t, all, 1)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	_, err := replicateHeaderRange(ctx, fx, hstore, 2, 50, 1, 50*time.Millisecond, chainID, nil)
	if err == nil {
		t.Fatal("expected error on cancel, got nil")
	}
}

// TestFetchLoopRejectsWrongChainID confirms validateChunkChainID kicks in.
func TestFetchLoopRejectsWrongChainID(t *testing.T) {
	suite := headertest.NewTestSuite(t, 3, 0)
	all := suite.GenExtendedHeaders(64)

	fx := &fakeExchange{headers: all}
	hstore, cleanup := newReplicationFixture(t, all, 0)
	defer cleanup()

	_, err := replicateHeaderRange(
		context.Background(), fx, hstore,
		1, 64, 2, time.Second, "wrong-chain", nil,
	)
	if err == nil {
		t.Fatal("expected chain-id mismatch error")
	}
}

// TestFetchLoopRejectsNonAdjacentChunk corrupts the returned chunk so its
// first header does not extend the local tip.
func TestFetchLoopRejectsNonAdjacentChunk(t *testing.T) {
	suite := headertest.NewTestSuite(t, 3, 0)
	all := suite.GenExtendedHeaders(200)
	chainID := all[0].ChainID()

	// Build an independent chain whose hash differs from `all`. Returning a
	// range from this chain still has correct heights but breaks adjacency
	// against the locally trusted tip seeded from `all`.
	otherSuite := headertest.NewTestSuite(t, 3, 0)
	other := otherSuite.GenExtendedHeaders(200)

	fx := &fakeExchange{headers: other, chainID: chainID}
	hstore, cleanup := newReplicationFixture(t, all, 10) // seeded from `all`
	defer cleanup()

	_, err := replicateHeaderRange(
		context.Background(), fx, hstore,
		11, 64, 1, time.Second, chainID, nil,
	)
	if err == nil {
		t.Fatal("expected non-adjacent chunk error")
	}
}

// TestChunkMath verifies the chunk splitter rounds and clamps correctly.
func TestChunkMath(t *testing.T) {
	if got := countChunks(1, 64, 64); got != 1 {
		t.Errorf("countChunks(1,64,64)=%d, want 1", got)
	}
	if got := countChunks(1, 65, 64); got != 2 {
		t.Errorf("countChunks(1,65,64)=%d, want 2", got)
	}
	if got := countChunks(1, 200, 64); got != 4 {
		t.Errorf("countChunks(1,200,64)=%d, want 4", got)
	}

	c := chunkByIndex(1, 200, 64, 3)
	if c.start != 193 || c.end != 200 {
		t.Errorf("chunkByIndex tail: got %+v, want start=193 end=200", c)
	}
	c = chunkByIndex(1, 64, 64, 0)
	if c.start != 1 || c.end != 64 {
		t.Errorf("chunkByIndex full: got %+v", c)
	}
}
