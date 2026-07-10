package headers

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	dsbadger "github.com/ipfs/go-ds-badger4"

	libhead_store "github.com/celestiaorg/go-header/store"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
)

// seededResumeStore builds a header store the way a prior replicate run leaves
// one: the first `seed` headers are appended, flushed, and their tip published
// as /headers/head, then the store is reopened. This makes hstore.Head() return
// the seeded tip exactly as it does on a real resumed run (an in-memory Append
// alone does not — the head pointer is only readable once persisted).
func seededResumeStore(t *testing.T, all []*header.ExtendedHeader, seed int) (
	*libhead_store.Store[*header.ExtendedHeader],
	func(),
) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "data")
	ds, err := dsbadger.NewDatastore(dir, nil)
	if err != nil {
		t.Fatalf("open badger: %v", err)
	}

	if seed > 0 {
		st, err := libhead_store.NewStore[*header.ExtendedHeader](ds, libhead_store.WithWriteBatchSize(64))
		if err != nil {
			t.Fatalf("new store: %v", err)
		}
		if err := st.Start(context.Background()); err != nil {
			t.Fatalf("start store: %v", err)
		}
		if err := st.Append(context.Background(), all[:seed]...); err != nil {
			t.Fatalf("seed append: %v", err)
		}
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := st.Stop(stopCtx); err != nil { // flush height entries to disk
			cancel()
			t.Fatalf("stop store: %v", err)
		}
		cancel()
		if err := PersistHeadKey(ds, all[seed-1]); err != nil {
			t.Fatalf("persist head: %v", err)
		}
	}

	hstore, err := libhead_store.NewStore[*header.ExtendedHeader](ds, libhead_store.WithWriteBatchSize(64))
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	if err := hstore.Start(context.Background()); err != nil {
		t.Fatalf("restart store: %v", err)
	}
	cleanup := func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = hstore.Stop(stopCtx)
		_ = ds.Close()
	}
	return hstore, cleanup
}

// TestResolveRangeSkipsPresentHeaders asserts resolveRange never re-downloads
// headers already contiguously present in the store: start is clamped up to
// head+1 whether or not an explicit --from-height was given, while a forward gap
// above head+1 is preserved.
func TestResolveRangeSkipsPresentHeaders(t *testing.T) {
	suite := headertest.NewTestSuite(t, 3, 0)
	all := suite.GenExtendedHeaders(300)

	const srcHead = 300
	cases := []struct {
		name      string
		seed      int // headers durably present in the store (head == seed)
		fromFlag  uint64
		toFlag    uint64
		wantStart uint64
		wantTgt   uint64
	}{
		{name: "empty store, no from -> genesis", seed: 0, fromFlag: 0, toFlag: 0, wantStart: 1, wantTgt: srcHead},
		{name: "empty store, explicit from honored", seed: 0, fromFlag: 50, toFlag: 200, wantStart: 50, wantTgt: 200},
		{name: "resume from head+1 when no from", seed: 100, fromFlag: 0, toFlag: 0, wantStart: 101, wantTgt: srcHead},
		{name: "from below head clamped up (no re-download)", seed: 100, fromFlag: 30, toFlag: 0, wantStart: 101, wantTgt: srcHead},
		{name: "from exactly head+1 kept", seed: 100, fromFlag: 101, toFlag: 250, wantStart: 101, wantTgt: 250},
		{name: "forward gap above head+1 preserved", seed: 100, fromFlag: 150, toFlag: 250, wantStart: 150, wantTgt: 250},
		{name: "fully caught up -> start past target", seed: 300, fromFlag: 0, toFlag: 0, wantStart: 301, wantTgt: srcHead},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hstore, cleanup := seededResumeStore(t, all, tc.seed)
			defer cleanup()

			start, target, err := resolveRange(context.Background(), hstore, tc.fromFlag, tc.toFlag, srcHead)
			if err != nil {
				t.Fatalf("resolveRange: %v", err)
			}
			if start != tc.wantStart {
				t.Errorf("start = %d, want %d", start, tc.wantStart)
			}
			if target != tc.wantTgt {
				t.Errorf("target = %d, want %d", target, tc.wantTgt)
			}
		})
	}
}

// TestResolveRangeRejectsTargetAboveSource keeps the existing guard: a --to-height
// beyond the source head is an error.
func TestResolveRangeRejectsTargetAboveSource(t *testing.T) {
	suite := headertest.NewTestSuite(t, 3, 0)
	all := suite.GenExtendedHeaders(10)

	hstore, cleanup := seededResumeStore(t, all, 0)
	defer cleanup()

	if _, _, err := resolveRange(context.Background(), hstore, 0, 500, 300); err == nil {
		t.Fatal("expected error for to-height above source head")
	}
}
